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
	"errors"
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
	"code.uplex.de/uplex-varnish/varnishapi/pkg/vsm"
)

// XXX timeout for getting Admin connection (waiting for varnishd start)
// timeout waiting for child process to stop
const (
	vclDir       = "/etc/varnish"
	vclFile      = "ingress.vcl"
	varnishLsn   = ":80"
	varnishdPath = "/usr/sbin/varnishd"
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
	varnishArgs = []string{"-a", varnishLsn, "-f", vclPath, "-F"}
	vcacheUID   int
	varnishGID  int
	currentIng  string
	configCtr   = uint64(0)
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

	vc.varnishdCmd = exec.Command(varnishdPath, varnishArgs...)
	if err := vc.varnishdCmd.Start(); err != nil {
		vc.errChan <- err
		return
	}
	log.Print("Launched varnishd")

	// XXX config the timeout
	vsm := vsm.New()
	if vsm == nil {
		vc.errChan <- errors.New("Cannot initiate attachment to "+
			"Varnish shared memory")
		return
	}
	defer vsm.Destroy()
	if err := vsm.Attach(""); err != nil {
		vc.errChan <- err
		return
	}
	addr, err := vsm.GetMgmtAddr()
	if err != nil {
		vc.errChan <- err
		return
	}
	spath, err := vsm.GetSecretPath()
	if err != nil {
		vc.errChan <- err
		return
	}
	sfile, err := os.Open(spath)
	if err != nil {
		vc.errChan <- err
		return
	}
	secret, err := ioutil.ReadAll(sfile)
	if err != nil {
		vc.errChan <- err
		return
	}
	if vc.adm, err = admin.Dial(addr, secret, 10*time.Second); err != nil {
		vc.errChan <- err
		return
	}
	log.Print("Got varnish admin connection")
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
