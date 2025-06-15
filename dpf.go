package go2dpf

import (
	"fmt"
	"github.com/google/gousb"
	"log"
)

const (
	ax206vid          = 0x1908 // Hacked frames USB Vendor ID
	ax206pid          = 0x0102 // Hacked frames USB Product ID
	endpOut           = 0x01
	endpIn            = 0x81
	intfNum           = 0
	cfgNum            = 1
	scsiTimeout       = 1000 // milliseconds
	usbCmdSetProperty = 0x01 // USB command: Set property
	usbCmdBlit        = 0x12 // USB command: Blit to screen
)

type DPF struct {
	Width  int
	Height int
	Debug  bool

	ctx   *gousb.Context
	dev   *gousb.Device
	intf  *gousb.Interface
	epOut *gousb.OutEndpoint
	epIn  *gousb.InEndpoint
}

func OpenDpf() (*DPF, error) {
	ctx := gousb.NewContext()

	dev, err := ctx.OpenDeviceWithVIDPID(ax206vid, ax206pid)
	if err != nil {
		ctx.Close()
		return nil, fmt.Errorf("Failed to open device: %v", err)
	}
	if dev == nil {
		ctx.Close()
		return nil, fmt.Errorf("Device %04x:%04x not found", ax206vid, ax206pid)
	}

	if err := dev.SetAutoDetach(true); err != nil {
		dev.Close()
		ctx.Close()
		return nil, fmt.Errorf("Failed to auto-detach kernel driver: %v", err)
	}

	cfg, err := dev.Config(cfgNum)
	if err != nil {
		dev.Close()
		ctx.Close()
		return nil, fmt.Errorf("Failed to set config: %v", err)
	}

	intf, err := cfg.Interface(intfNum, 0)
	if err != nil {
		cfg.Close()
		dev.Close()
		ctx.Close()
		return nil, fmt.Errorf("Failed to claim interface: %v", err)
	}

	epOut, err := intf.OutEndpoint(endpOut)
	if err != nil {
		intf.Close()
		cfg.Close()
		dev.Close()
		ctx.Close()
		return nil, fmt.Errorf("Failed to open OUT endpoint: %v", err)
	}

	epIn, err := intf.InEndpoint(endpIn & 0x0f)
	if err != nil {
		intf.Close()
		cfg.Close()
		dev.Close()
		ctx.Close()
		return nil, fmt.Errorf("Failed to open IN endpoint: %v", err)
	}

	return &DPF{
		ctx:   ctx,
		dev:   dev,
		intf:  intf,
		epOut: epOut,
		epIn:  epIn,
	}, nil
}

func (d *DPF) Close() {
	if d.intf != nil {
		d.intf.Close()
	}
	if d.dev != nil {
		d.dev.Close()
	}
	if d.ctx != nil {
		d.ctx.Close()
	}
}

func (dpf *DPF) GetDimensions() (width, height int, err error) {
	cmd := []byte{
		0xcd, 0, 0, 0,
		0, 2, 0, 0,
		0, 0, 0, 0,
		0, 0, 0, 0,
	}
	data, err := dpf.scsiRead(cmd, 5)
	if err != nil {
		return 0, 0, err
	}
	width = int(data[0]) | int(data[1])<<8
	height = int(data[2]) | int(data[3])<<8
	return width, height, nil
}

func (dpf *DPF) Brightness(lvl int) error {
	if lvl < 0 {
		lvl = 0
	}
	if lvl > 7 {
		lvl = 7
	}

	cmd := []byte{
		0xcd, 0, 0, 0,
		0, 6, usbCmdSetProperty,
		1, 0, // PROPERTY_BRIGHTNESS
		byte(lvl), byte(lvl >> 8),
		0, 0, 0, 0, 0,
	}

	return dpf.scsiWrite(cmd, nil)
}

func (dpf *DPF) Blit(img *ImageRGB565) error {

	r := img.Rect
	cmd := []byte{
		0xcd, 0, 0, 0,
		0, 6, usbCmdBlit,
		byte(r.Min.X), byte(r.Min.X >> 8),
		byte(r.Min.Y), byte(r.Min.Y >> 8),
		byte(r.Max.X - 1), byte((r.Max.X - 1) >> 8),
		byte(r.Max.Y - 1), byte((r.Max.Y - 1) >> 8),
		0,
	}
	return dpf.scsiWrite(cmd, img.PixRect())
}

func (d *DPF) scsiCmdPrepare(cmd []byte, blockLen int, out bool) []byte {
	var bmCBWFlags byte
	if out {
		bmCBWFlags = 0x00
	} else {
		bmCBWFlags = 0x80
	}
	buf := []byte{
		0x55, 0x53, 0x42, 0x43, // USBC
		0xde, 0xad, 0xbe, 0xef, // Tag
		byte(blockLen), byte(blockLen >> 8), byte(blockLen >> 16), byte(blockLen >> 24),
		bmCBWFlags,
		0x00, byte(len(cmd)),
	}
	// SCSI Command (16 bytes total)
	cmdBuf := make([]byte, 16)
	copy(cmdBuf, cmd)
	buf = append(buf, cmdBuf...)

	if d.Debug {
		log.Printf("SCSI CMD PREP: %x", buf)
	}
	return buf
}

func (d *DPF) scsiGetAck() error {
	buf := make([]byte, 13)
	n, err := d.epIn.Read(buf)
	if err != nil {
		return fmt.Errorf("Failed to read ACK: %w", err)
	}
	if n < 4 || string(buf[:4]) != "USBS" {
		return fmt.Errorf("Invalid ACK: %x", buf[:n])
	}
	if d.Debug {
		log.Printf("Got ACK: %x", buf[:n])
	}
	return nil
}

func (d *DPF) scsiWrite(cmd []byte, data []byte) error {
	header := d.scsiCmdPrepare(cmd, len(data), true)
	if _, err := d.epOut.Write(header); err != nil {
		return fmt.Errorf("Failed to send SCSI command: %w", err)
	}
	if data != nil {
		if _, err := d.epOut.Write(data); err != nil {
			return fmt.Errorf("Failed to send SCSI data: %w", err)
		}
	}
	return d.scsiGetAck()
}

func (d *DPF) scsiRead(cmd []byte, blockLen int) ([]byte, error) {
	header := d.scsiCmdPrepare(cmd, blockLen, false)
	if _, err := d.epOut.Write(header); err != nil {
		return nil, fmt.Errorf("Failed to send SCSI read command: %w", err)
	}

	buf := make([]byte, blockLen)
	n, err := d.epIn.Read(buf)
	if err != nil {
		return nil, fmt.Errorf("Failed to read SCSI data: %w", err)
	}
	if d.Debug {
		log.Printf("Read data: %x", buf[:n])
	}
	if err := d.scsiGetAck(); err != nil {
		return buf[:n], err
	}
	return buf[:n], nil
}
