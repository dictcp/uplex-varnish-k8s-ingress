package vcl

import (
	"bytes"
	"testing"
)

var teaSvc = Service{
	Name: "tea-svc",
	Addresses: []Address{
		{
			IP:   "192.0.2.1",
			Port: 80,
		},
		{
			IP:   "192.0.2.2",
			Port: 80,
		},
		{
			IP:   "192.0.2.3",
			Port: 80,
		},
	},
}

var coffeeSvc = Service{
	Name: "coffee-svc",
	Addresses: []Address{
		{
			IP:   "192.0.2.4",
			Port: 80,
		},
		{
			IP:   "192.0.2.5",
			Port: 80,
		},
	},
}

var cafeSpec = Spec{
	DefaultService: Service{},
	Rules: []Rule{{
		Host: "cafe.example.com",
		PathMap: map[string]Service{
			"/tea":    teaSvc,
			"/coffee": coffeeSvc,
		},
	}},
	AllServices: map[string]Service{
		"tea-svc":    teaSvc,
		"coffee-svc": coffeeSvc,
	},
}

func TestTemplate(t *testing.T) {
	var buf bytes.Buffer
	if err := Tmpl.Execute(&buf, cafeSpec); err != nil {
		t.Error("Execute():", err)
	}
	t.Log(string(buf.Bytes()))
}
