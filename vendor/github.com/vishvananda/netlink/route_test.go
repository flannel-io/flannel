package netlink

import (
	"net"
	"syscall"
	"testing"
	"time"
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
		t.Fatal("Link not added properly")
	}

	dstIP := net.ParseIP("192.168.0.42")
	routeToDstIP, err := RouteGet(dstIP)
	if err != nil {
		t.Fatal(err)
	}

	if len(routeToDstIP) == 0 {
		t.Fatal("Default route not present")
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

func TestRouteAddIncomplete(t *testing.T) {
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

	route := Route{LinkIndex: link.Attrs().Index}
	if err := RouteAdd(&route); err == nil {
		t.Fatal("Adding incomplete route should fail")
	}
}

func expectRouteUpdate(ch <-chan RouteUpdate, t uint16, dst net.IP) bool {
	for {
		timeout := time.After(time.Minute)
		select {
		case update := <-ch:
			if update.Type == t && update.Route.Dst.IP.Equal(dst) {
				return true
			}
		case <-timeout:
			return false
		}
	}
}

func TestRouteSubscribe(t *testing.T) {
	tearDown := setUpNetlinkTest(t)
	defer tearDown()

	ch := make(chan RouteUpdate)
	done := make(chan struct{})
	defer close(done)
	if err := RouteSubscribe(ch, done); err != nil {
		t.Fatal(err)
	}

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

	if !expectRouteUpdate(ch, syscall.RTM_NEWROUTE, dst.IP) {
		t.Fatal("Add update not received as expected")
	}

	err = RouteDel(&route)
	if err != nil {
		t.Fatal(err)
	}

	if !expectRouteUpdate(ch, syscall.RTM_DELROUTE, dst.IP) {
		t.Fatal("Del update not received as expected")
	}
}
