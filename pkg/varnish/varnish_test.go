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

package varnish

import (
	"fmt"
	"testing"
)

func TestVarnishAdmError(t *testing.T) {
	vadmErr := VarnishAdmError{
		addr: "123.45.67.89:4711",
		err:  fmt.Errorf("Error message"),
	}
	err := vadmErr.Error()
	want := "123.45.67.89:4711: Error message"
	if err != want {
		t.Errorf("VarnishAdmError.Error() want=%s got=%s", want, err)
	}

	vadmErrs := VarnishAdmErrors{
		vadmErr,
		VarnishAdmError{
			addr: "98.76.54.321:815",
			err:  fmt.Errorf("Error 2"),
		},
		VarnishAdmError{
			addr: "192.0.2.255:80",
			err:  fmt.Errorf("Error 3"),
		},
	}
	err = vadmErrs.Error()
	want = "[{123.45.67.89:4711: Error message}{98.76.54.321:815: Error 2}" +
		"{192.0.2.255:80: Error 3}]"
	if err != want {
		t.Errorf("VarnishAdmErrors.Error() want=%s got=%s", want, err)
	}
}
