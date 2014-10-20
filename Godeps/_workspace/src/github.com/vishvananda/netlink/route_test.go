package netlink

import (
	"net"
	"testing"
)

func TestRouteAddDel(t *testing.T) {
	tearDown := setUpNetlinkTest(t)
	defer tearDown()

	// get loopback interface
	link, err := LinkByName("lo")
	if err != nil {
		t.Fatal(err)
	}

	// bring the interface up
	if err = LinkSetUp(link); err != nil {
		t.Fatal(err)
	}

	// add a gateway route
	_, dst, err := net.ParseCIDR("192.168.0.0/24")

	ip := net.ParseIP("127.1.1.1")
	route := Route{LinkIndex: link.Attrs().Index, Dst: dst, Src: ip}
	err = RouteAdd(&route)
	if err != nil {
		t.Fatal(err)
	}
	routes, err := RouteList(link, FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 {
		t.Fatal("Link not removed properly")
	}

	err = RouteDel(&route)
	if err != nil {
		t.Fatal(err)
	}

	routes, err = RouteList(link, FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 0 {
		t.Fatal("Route not removed properly")
	}

}
