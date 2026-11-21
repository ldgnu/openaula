package aula

// Conn identifies how the keyboard is currently reachable.
type Conn string

const (
	// ConnNone means no keyboard (or dongle) was found.
	ConnNone Conn = "none"
	// ConnUSB is the wired USB-C connection, which supports the full
	// configuration protocol.
	ConnUSB Conn = "usb"
	// Conn24G is the 2.4 GHz dongle. Its vendor channel is not documented and
	// sending configuration packets over it can hang the dongle, so openaula
	// never writes to it.
	Conn24G Conn = "2.4g"
)

// DongleVID/PID: the 2.4 GHz receiver spoofs an Apple keyboard.
const (
	DongleVendorID  = 0x05AC
	DongleProductID = 0x024F
)

// Info describes the detected device.
type Info struct {
	Connected     bool   `json:"connected"`
	Transport     Conn   `json:"transport"`
	VID           uint16 `json:"vid"`
	PID           uint16 `json:"pid"`
	Product       string `json:"product"`
	Manufacturer  string `json:"manufacturer,omitempty"`
	Bus           int    `json:"bus"`
	Address       int    `json:"address"`
	Serial        string `json:"serial,omitempty"`
	BCDDevice     uint16 `json:"bcd_device"`
	NumInterfaces int    `json:"num_interfaces"`
	CanConfigure  bool   `json:"can_configure"`
}

// Detect scans the USB bus for a wired F108 Pro or its 2.4 GHz dongle.
// A wired keyboard always wins over the dongle.
func Detect() Info {
	if info, ok := detectPlatform(VendorID, ProductID); ok {
		info.Transport = ConnUSB
		info.CanConfigure = true
		return info
	}
	if info, ok := detectDongle(); ok {
		return info
	}
	return Info{Transport: ConnNone}
}

// FirmwareVersion returns the firmware number the vendor UI shows, derived
// from bcdDevice (0x0120 -> "120").
func (i Info) FirmwareVersion() string {
	if i.BCDDevice == 0 {
		return ""
	}
	return FirmwareFromBCD(i.BCDDevice)
}

// IsConfigurable reports whether the given connection supports configuration.
func (c Conn) IsConfigurable() bool { return c == ConnUSB || c == Conn24G }
