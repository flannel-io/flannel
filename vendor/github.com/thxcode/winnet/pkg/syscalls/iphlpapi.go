package syscalls

import (
	"syscall"
	"unsafe"
)

var (
	modiphlpapi = syscall.NewLazyDLL("iphlpapi.dll")

	procCreateIpForwardEntry = modiphlpapi.NewProc("CreateIpForwardEntry")
	procDeleteIpForwardEntry = modiphlpapi.NewProc("DeleteIpForwardEntry")
	procGetIpForwardTable    = modiphlpapi.NewProc("GetIpForwardTable")
	procGetIpInterfaceEntry  = modiphlpapi.NewProc("GetIpInterfaceEntry")
	procSetIpInterfaceEntry  = modiphlpapi.NewProc("SetIpInterfaceEntry")
)

// https://docs.microsoft.com/en-us/windows/win32/api/ipmib/ns-ipmib-mib_ipforwardrow
// typedef struct _MIB_IPFORWARDROW {
//  DWORD    dwForwardDest;
//  DWORD    dwForwardMask;
//  DWORD    dwForwardPolicy;
//  DWORD    dwForwardNextHop;
//  IF_INDEX dwForwardIfIndex;
//  union {
//    DWORD              dwForwardType;
//    MIB_IPFORWARD_TYPE ForwardType;
//  };
//  union {
//    DWORD               dwForwardProto;
//    MIB_IPFORWARD_PROTO ForwardProto;
//  };
//  DWORD    dwForwardAge;
//  DWORD    dwForwardNextHopAS;
//  DWORD    dwForwardMetric1;
//  DWORD    dwForwardMetric2;
//  DWORD    dwForwardMetric3;
//  DWORD    dwForwardMetric4;
//  DWORD    dwForwardMetric5;
// }
type MibIpForwardRow struct {
	ForwardDest      uint32
	ForwardMask      uint32
	ForwardPolicy    uint32
	ForwardNextHop   uint32
	ForwardIfIndex   uint32
	ForwardType      uint32
	ForwardProto     uint32
	ForwardAge       uint32
	ForwardNextHopAS uint32
	ForwardMetric1   uint32
	ForwardMetric2   uint32
	ForwardMetric3   uint32
	ForwardMetric4   uint32
	ForwardMetric5   uint32
}

func CreateIpForwardEntry(pIpForwardRow *MibIpForwardRow) (errcode error) {
	r0, _, _ := syscall.Syscall(procCreateIpForwardEntry.Addr(), 1, uintptr(unsafe.Pointer(pIpForwardRow)), 0, 0)
	if r0 != 0 {
		errcode = syscall.Errno(r0)
	}
	return
}

func DeleteIpForwardEntry(pIpForwardTable *MibIpForwardRow) (errcode error) {
	r0, _, _ := syscall.Syscall(procDeleteIpForwardEntry.Addr(), 1, uintptr(unsafe.Pointer(pIpForwardTable)), 0, 0)
	if r0 != 0 {
		errcode = syscall.Errno(r0)
	}
	return
}

// https://docs.microsoft.com/en-us/windows/win32/api/ipmib/ns-ipmib-mib_ipforwardtable
// typedef struct _MIB_IPFORWARDTABLE {
//  DWORD            dwNumEntries;
//  MIB_IPFORWARDROW table[ANY_SIZE];
// }
type MibIpForwardTable struct {
	NumEntries uint32
	Table      [1]MibIpForwardRow
}

func GetIpForwardTable(pIpForwardTable *MibIpForwardTable, pSize *uint32, order bool) (errcode error) {
	var _p0 uint32
	if order {
		_p0 = 1
	} else {
		_p0 = 0
	}
	r0, _, _ := syscall.Syscall(procGetIpForwardTable.Addr(), 3, uintptr(unsafe.Pointer(pIpForwardTable)), uintptr(unsafe.Pointer(pSize)), uintptr(_p0))
	if r0 != 0 {
		errcode = syscall.Errno(r0)
	}
	return
}

// https://docs.microsoft.com/en-us/windows/win32/api/nldef/ns-nldef-nl_interface_offload_rod
type NLInterfaceOffloadRod [8]byte

// https://docs.microsoft.com/en-us/windows/win32/api/netioapi/ns-netioapi-mib_ipinterface_row
//typedef struct _MIB_IPINTERFACE_ROW {
//  ADDRESS_FAMILY                 Family;
//  NET_LUID                       InterfaceLuid;
//  NET_IFINDEX                    InterfaceIndex;
//  ULONG                          MaxReassemblySize;
//  ULONG64                        InterfaceIdentifier;
//  ULONG                          MinRouterAdvertisementInterval;
//  ULONG                          MaxRouterAdvertisementInterval;
//  BOOLEAN                        AdvertisingEnabled;
//  BOOLEAN                        ForwardingEnabled;
//  BOOLEAN                        WeakHostSend;
//  BOOLEAN                        WeakHostReceive;
//  BOOLEAN                        UseAutomaticMetric;
//  BOOLEAN                        UseNeighborUnreachabilityDetection;
//  BOOLEAN                        ManagedAddressConfigurationSupported;
//  BOOLEAN                        OtherStatefulConfigurationSupported;
//  BOOLEAN                        AdvertiseDefaultRoute;
//  NL_ROUTER_DISCOVERY_BEHAVIOR   RouterDiscoveryBehavior;
//  ULONG                          DadTransmits;
//  ULONG                          BaseReachableTime;
//  ULONG                          RetransmitTime;
//  ULONG                          PathMtuDiscoveryTimeout;
//  NL_LINK_LOCAL_ADDRESS_BEHAVIOR LinkLocalAddressBehavior;
//  ULONG                          LinkLocalAddressTimeout;
//  ULONG                          ZoneIndices[ScopeLevelCount];
//  ULONG                          SitePrefixLength;
//  ULONG                          Metric;
//  ULONG                          NlMtu;
//  BOOLEAN                        Connected;
//  BOOLEAN                        SupportsWakeUpPatterns;
//  BOOLEAN                        SupportsNeighborDiscovery;
//  BOOLEAN                        SupportsRouterDiscovery;
//  ULONG                          ReachableTime;
//  NL_INTERFACE_OFFLOAD_ROD       TransmitOffload;
//  NL_INTERFACE_OFFLOAD_ROD       ReceiveOffload;
//  BOOLEAN                        DisableDefaultRoutes;
//}
type MibIpInterfaceRow struct {
	Family                               uint32
	InterfaceLuid                        uint64
	InterfaceIndex                       uint32
	MaxReassemblySize                    uint32
	InterfaceIdentifier                  uint64
	MinRouterAdvertisementInterval       uint32
	MaxRouterAdvertisementInterval       uint32
	AdvertisingEnabled                   uint8
	ForwardingEnabled                    uint8
	WeakHostSend                         uint8
	WeakHostReceive                      uint8
	UseAutomaticMetric                   uint8
	UseNeighborUnreachabilityDetection   uint8
	ManagedAddressConfigurationSupported uint8
	OtherStatefulConfigurationSupported  uint8
	AdvertiseDefaultRoute                uint8
	RouterDiscoveryBehavior              uint32
	DadTransmits                         uint32
	BaseReachableTime                    uint32
	RetransmitTime                       uint32
	PathMtuDiscoveryTimeout              uint32
	LinkLocalAddressBehavior             uint32
	LinkLocalAddressTimeout              uint32
	ZoneIndices                          [16]uint32
	SitePrefixLength                     uint32
	Metric                               uint32
	NlMtu                                uint32
	Connected                            uint8
	SupportsWakeUpPatterns               uint8
	SupportsNeighborDiscovery            uint8
	SupportsRouterDiscovery              uint8
	ReachableTime                        uint32
	TransmitOffload                      uint8
	ReceiveOffload                       uint8
	DisableDefaultRoutes                 uint8
}

func GetIpInterfaceEntry(pIfRow *MibIpInterfaceRow) (errcode error) {
	r0, _, _ := syscall.Syscall(procGetIpInterfaceEntry.Addr(), 1, uintptr(unsafe.Pointer(pIfRow)), 0, 0)
	if r0 != 0 {
		errcode = syscall.Errno(r0)
	}
	return
}

func SetIpInterfaceEntry(pIfRow *MibIpInterfaceRow) (errcode error) {
	r0, _, _ := syscall.Syscall(procSetIpInterfaceEntry.Addr(), 1, uintptr(unsafe.Pointer(pIfRow)), 0, 0)
	if r0 != 0 {
		errcode = syscall.Errno(r0)
	}
	return
}
