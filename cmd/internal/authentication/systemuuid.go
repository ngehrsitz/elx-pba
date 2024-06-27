package authentication

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/u-root/u-root/pkg/smbios"
)

type SystemUUIDAuthenticator struct{}

func (s SystemUUIDAuthenticator) RetrievePassword() (string, error) {
	dmi, err := readDMI()
	if err != nil {
		return "", fmt.Errorf("Failed to read SMBIOS/DMI data: %v", err)
	}

	log.Printf("System UUID:            %s", dmi.SystemUUID)
	log.Printf("System serial:          %s", dmi.SystemSerialNumber)
	log.Printf("Baseboard manufacturer: %s", dmi.BaseboardManufacturer)
	log.Printf("Baseboard product:      %s", dmi.BaseboardProduct)
	log.Printf("Baseboard serial:       %s", dmi.BaseboardSerialNumber)
	log.Printf("Chassis serial:         %s", dmi.ChassisSerialNumber)

	return fmt.Sprintf("%s", dmi.SystemUUID), nil
}

type DMIData struct {
	SystemUUID            string
	SystemSerialNumber    string
	BaseboardManufacturer string
	BaseboardProduct      string
	BaseboardSerialNumber string
	ChassisSerialNumber   string
}

func readDMI() (*DMIData, error) {
	sysfsPath := "/sys/firmware/dmi/tables"
	smbiosPath := filepath.Join(sysfsPath, "smbios")
	entry, smbiosErr := os.ReadFile(smbiosPath)
	if smbiosErr != nil {
		return nil, fmt.Errorf("failed to read %q: %v", smbiosPath, smbiosErr)
	}
	tablePath := filepath.Join(sysfsPath, "DMI")
	table, tableErr := os.ReadFile(tablePath)
	if tableErr != nil {
		return nil, fmt.Errorf("failed to read %q: %v", tablePath, tableErr)
	}

	si, parseErr := smbios.ParseInfo(entry, table)
	if parseErr != nil {
		return nil, fmt.Errorf("failed to parse SMBIOS info: %v", parseErr)
	}

	dmi := &DMIData{}
	for _, t := range si.Tables {
		pt, err := smbios.ParseTypedTable(t)
		if err != nil {
			continue
		}
		if ci, ok := pt.(*smbios.ChassisInfo); ok {
			dmi.ChassisSerialNumber = ci.SerialNumber
		} else if bi, ok := pt.(*smbios.BaseboardInfo); ok {
			dmi.BaseboardManufacturer = bi.Manufacturer
			dmi.BaseboardProduct = bi.Product
			dmi.BaseboardSerialNumber = bi.SerialNumber
		} else if si, ok := pt.(*smbios.SystemInfo); ok {
			dmi.SystemSerialNumber = si.SerialNumber
			dmi.SystemUUID = si.UUID.String()
		}
	}

	return dmi, nil
}
