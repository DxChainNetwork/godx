package storagehosttree

import "testing"

var ips = []string{
	"99.0.86.9",
	"104.143.92.125",
	"104.237.91.15",
	"185.192.69.89",
	"104.238.46.146",
	"104.238.46.156",
}

var filter = NewFilter()

func TestFilter_Filtered(t *testing.T) {

	for _, ip := range ips {
		filter.Add(ip)
	}

	for _, ip := range ips {
		out := filter.Filtered(ip)
		if out != true {
			t.Errorf("error: the ip address %s should be filtered", ip)
		}
	}
}

func TestFilter_Reset(t *testing.T) {
	filter.Reset()

	for _, ip := range ips {
		out := filter.Filtered(ip)
		if out != false {
			t.Errorf("error: the ip address %s should not be filtered", ip)
		}
	}
}
