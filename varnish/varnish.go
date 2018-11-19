/*
 * Copyright (c) 2018 UPLEX Nils Goroll Systemoptimierung
 * All rights reserved
 *
 * Author: Geoffrey Simmons <geoffrey.simmons@uplex.de>
 *
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted provided that the following conditions
 * are met:
 * 1. Redistributions of source code must retain the above copyright
 *    notice, this list of conditions and the following disclaimer.
 * 2. Redistributions in binary form must reproduce the above copyright
 *    notice, this list of conditions and the following disclaimer in the
 *    documentation and/or other materials provided with the distribution.
 *
 * THIS SOFTWARE IS PROVIDED BY THE AUTHOR AND CONTRIBUTORS ``AS IS'' AND
 * ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
 * IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
 * ARE DISCLAIMED.  IN NO EVENT SHALL AUTHOR OR CONTRIBUTORS BE LIABLE
 * FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL
 * DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS
 * OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION)
 * HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT
 * LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY
 * OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF
 * SUCH DAMAGE.
 */

/*
// TODO
* VCL housekeeping
  * either discard the previously active VCL immediately on new vcl.use
  * or periodically clean up

* monitoring
  * periodically call ping, status, panic.show when otherwise idle
*/

package varnish

import (
	"bufio"
	"crypto/rand"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	"code.uplex.de/uplex-varnish/k8s-ingress/varnish/vcl"

	"code.uplex.de/uplex-varnish/varnishapi/pkg/admin"
)

// XXX timeout for getting Admin connection (waiting for varnishd start)
// timeout waiting for child process to stop
const (
	vclDir       = "/etc/varnish"
	vclFile      = "ingress.vcl"
	varnishLsn   = ":80"
	varnishdPath = "/usr/sbin/varnishd"
	admConn      = "localhost:6081"
	secretFile   = "_.secret"
	notFoundVCL  = `vcl 4.0;

backend default { .host = "192.0.2.255"; .port = "80"; }

sub vcl_recv {
	return (synth(404));
}
`
)

var (
	vclPath     = filepath.Join(vclDir, vclFile)
	tmpPath     = filepath.Join(os.TempDir(), vclFile)
	secretPath  = filepath.Join(vclDir, secretFile)
	varnishArgs = []string{
		"-a", varnishLsn, "-f", vclPath, "-F", "-S", secretPath,
		"-M", admConn,
	}
	vcacheUID  int
	varnishGID int
	currentIng string
	configCtr  = uint64(0)
)

type VarnishController struct {
	varnishdCmd *exec.Cmd
	adm         *admin.Admin
	errChan     chan error
}

func NewVarnishController() *VarnishController {
	return &VarnishController{}
}

func (vc *VarnishController) Start(errChan chan error) {
	vc.errChan = errChan

	log.Print("Starting Varnish controller")
	vcacheUser, err := user.Lookup("varnish")
	if err != nil {
		vc.errChan <- err
		return
	}
	varnishGrp, err := user.LookupGroup("varnish")
	if err != nil {
		vc.errChan <- err
		return
	}
	vcacheUID, err = strconv.Atoi(vcacheUser.Uid)
	if err != nil {
		vc.errChan <- err
		return
	}
	varnishGID, err = strconv.Atoi(varnishGrp.Gid)
	if err != nil {
		vc.errChan <- err
		return
	}

	secret := make([]byte, 32)
	_, err = rand.Read(secret)
	if err != nil {
		vc.errChan <- err
		return
	}
	if err := ioutil.WriteFile(secretPath, secret, 0400); err != nil {
		vc.errChan <- err
		return
	}
	if err := os.Chown(secretPath, vcacheUID, varnishGID); err != nil {
		vc.errChan <- err
		return
	}
	log.Print("Wrote secret file")

	notFoundBytes := []byte(notFoundVCL)
	if err := ioutil.WriteFile(vclPath, notFoundBytes, 0644); err != nil {
		vc.errChan <- err
		return
	}
	if err := os.Chown(vclPath, vcacheUID, varnishGID); err != nil {
		vc.errChan <- err
		return
	}
	log.Print("Wrote initial VCL file")

	if vc.adm, err = admin.Listen(admConn); err != nil {
		vc.errChan <- err
		return
	}
	log.Print("Opened port to listen for Varnish adm connection")

	vc.varnishdCmd = exec.Command(varnishdPath, varnishArgs...)
	if err := vc.varnishdCmd.Start(); err != nil {
		vc.errChan <- err
		return
	}
	log.Print("Launched varnishd")

	if err := vc.adm.Accept(secret); err != nil {
		vc.errChan <- err
		return
	}
	log.Print("Accepted varnish admin connection")
}

func (vc *VarnishController) Update(key string, spec vcl.Spec) error {
	if currentIng != "" && currentIng != key {
		return fmt.Errorf("Multiple Ingress definitions currently not "+
			"supported: current=%s new=%s", currentIng, key)
	}
	currentIng = key

	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	wr := bufio.NewWriter(f)
	if err := vcl.Tmpl.Execute(wr, spec); err != nil {
		return err
	}
	wr.Flush()
	f.Close()
	log.Printf("Wrote new VCL config to %s", tmpPath)

	ctr := atomic.AddUint64(&configCtr, 1)
	configName := fmt.Sprintf("ingress-%d", ctr)
	if err := vc.adm.VCLLoad(configName, tmpPath); err != nil {
		log.Print("Failed to load VCL: ", err)
		return err
	}
	log.Printf("Loaded VCL config %s", configName)

	newVCL, err := ioutil.ReadFile(tmpPath)
	if err != nil {
		log.Print(err)
		return err
	}
	if err = ioutil.WriteFile(vclPath, newVCL, 0644); err != nil {
		log.Print(err)
		return err
	}
	log.Printf("Wrote VCL config to %s", vclPath)

	if err = vc.adm.VCLUse(configName); err != nil {
		log.Print("Failed to activate VCL: ", err)
		return err
	}
	log.Printf("Activated VCL config %s", configName)

	// XXX discard previously active VCL
	return nil
}

// We currently only support one Ingress definition at a time, so
// deleting the Ingress means that we revert to the "boot" config,
// which returns synthetic 404 Not Found for all requests.
func (vc *VarnishController) DeleteIngress(key string) error {
	if currentIng != "" && currentIng != key {
		return fmt.Errorf("Unknown Ingress %s", key)
	}

	if err := vc.adm.VCLUse("boot"); err != nil {
		log.Print("Failed to activate VCL: ", err)
		return err
	}
	log.Printf("Activated VCL config boot")

	currentIng = ""
	// XXX discard previously active VCL
	return nil
}

// Currently only one Ingress at a time
func (vc *VarnishController) HasIngress(key string) bool {
	if currentIng == "" {
		return false
	}
	return key == currentIng
}

func (vc *VarnishController) Quit() {
	if err := vc.adm.Stop(); err != nil {
		log.Print("Failed to stop Varnish child process:", err)
	} else {
		for {
			tmoChan := time.After(time.Minute)
			select {
			case <-tmoChan:
				// XXX config the timeout
				log.Print("timeout waiting for Varnish child " +
					"process to finish")
				return
			default:
				state, err := vc.adm.Status()
				if err != nil {
					log.Print("Can't get Varnish child "+
						"process status:", err)
					return
				}
				if state != admin.Stopped {
					continue
				}
			}
		}
	}
	vc.adm.Close()

	if err := vc.varnishdCmd.Process.Signal(syscall.SIGTERM); err != nil {
		log.Print("Failed to stop Varnish:", err)
		return
	}
	log.Print("Stopped Varnish")
}
