package aula

import (
	"github.com/google/gousb"
)

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
	ctx := gousb.NewContext()
	defer ctx.Close()

	devs, err := ctx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		return isF108(desc)
	})
	if err != nil {
		return Info{Transport: ConnNone}
	}
	defer func() {
		for _, d := range devs {
			d.Close()
		}
	}()

	var usbInfo, dongleInfo *Info
	for _, d := range devs {
		desc := d.Desc
		info := Info{
			Connected:     true,
			VID:           uint16(desc.Vendor),
			PID:           uint16(desc.Product),
			Bus:           desc.Bus,
			Address:       desc.Address,
			BCDDevice:     uint16(desc.Device),
			NumInterfaces: len(desc.Configs[1].Interfaces),
		}
		if s, err := d.Product(); err == nil {
			info.Product = s
		}
		if s, err := d.Manufacturer(); err == nil {
			info.Manufacturer = s
		}
		if s, err := d.SerialNumber(); err == nil {
			info.Serial = s
		}

		if desc.Vendor == gousb.ID(VendorID) && desc.Product == gousb.ID(ProductID) {
			info.Transport = ConnUSB
			info.CanConfigure = true
			usbInfo = &info
		} else {
			info.Transport = Conn24G
			if info.Product == "" {
				info.Product = "F108Pro Dongle"
			}
			dongleInfo = &info
		}
	}

	switch {
	case usbInfo != nil:
		return *usbInfo
	case dongleInfo != nil:
		return *dongleInfo
	default:
		return Info{Transport: ConnNone}
	}
}

func isF108(desc *gousb.DeviceDesc) bool {
	if desc.Vendor == gousb.ID(VendorID) && desc.Product == gousb.ID(ProductID) {
		return true
	}
	return desc.Vendor == gousb.ID(DongleVendorID) && desc.Product == gousb.ID(DongleProductID)
}

// IsConfigurable reports whether the given connection supports configuration.
func (c Conn) IsConfigurable() bool { return c == ConnUSB }
