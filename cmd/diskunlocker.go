package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/elastx/elx-pba/cmd/internal/authentication"
	"github.com/elastx/elx-pba/cmd/internal/keyderiviation"

	tcg "github.com/open-source-firmware/go-tcg-storage/pkg/core"
	"github.com/open-source-firmware/go-tcg-storage/pkg/drive"
	"github.com/open-source-firmware/go-tcg-storage/pkg/locking"
	"golang.org/x/sys/unix"
)

type DiskUnlocker struct {
	auth authentication.Authenticator
	kdf  keyderiviation.KeyDerivationFunction
}

func (unlocker DiskUnlocker) UnlockDisks() (int, error) {
	unlockedDisks := 0
	sysblk, blockErr := os.ReadDir("/sys/class/block/")
	if blockErr != nil {
		return 0, fmt.Errorf("Failed to enumerate block devices: %v", blockErr)
	}

	password := ""
	for _, fi := range sysblk {
		devname := fi.Name()
		if _, err := os.Stat(filepath.Join("sys/class/block", devname, "device")); os.IsNotExist(err) {
			continue
		}
		devpath := filepath.Join("/dev", devname)
		if _, devErr := os.Stat(devpath); os.IsNotExist(devErr) {
			majmin, blockDevErr := os.ReadFile(filepath.Join("/sys/class/block", devname, "dev"))
			if blockDevErr != nil {
				log.Printf("Failed to read major:minor for %s: %v", devname, blockDevErr)
				continue
			}
			parts := strings.Split(strings.TrimSpace(string(majmin)), ":")
			major, _ := strconv.ParseInt(parts[0], 10, 8)
			minor, _ := strconv.ParseInt(parts[1], 10, 8)
			mknodErr := unix.Mknod(filepath.Join("/dev", devname), unix.S_IFBLK|0600, int(major<<16|minor))
			if mknodErr != nil {
				log.Printf("Mknod(%s) failed: %v", devname, mknodErr)
				continue
			}
		}

		drive, err := drive.Open(devpath)
		if err != nil {
			log.Printf("drive.Open(%s): %v", devpath, err)
			continue
		}
		defer drive.Close()
		identity, err := drive.Identify()
		if err != nil {
			log.Printf("drive.Identify(%s): %v", devpath, err)
		}
		dsn, err := drive.SerialNumber()
		if err != nil {
			log.Printf("drive.SerialNumber(%s): %v", devpath, err)
		}
		d0, err := tcg.Discovery0(drive)
		if err != nil {
			if err != tcg.ErrNotSupported {
				log.Printf("tcg.Discovery0(%s): %v", devpath, err)
			}
			continue
		}
		if d0.Locking != nil && d0.Locking.Locked {
			log.Printf("Drive %s is locked", identity)
			if d0.Locking.MBREnabled && !d0.Locking.MBRDone {
				log.Printf("Drive %s has active shadow MBR", identity)
			}
			unlocked := false
			for !unlocked {
				// reuse-existing password for multiple drives
				if password == "" {
					newPassword, passwordErr := unlocker.auth.RetrievePassword()
					if passwordErr != nil {
						return 0, fmt.Errorf("failed to retrieve password: %v", passwordErr)
					}
					password = newPassword
					if password == "" {
						// skip on empty password
						break
					}
				}
				unlockErr := unlocker.unlockDisk(drive, password, dsn)
				if unlockErr != nil {
					log.Printf("Failed to unlock %s: %v", identity, err)
					// clear password to be queried again
					password = ""
				} else {
					unlocked = true
				}
			}
			if unlocked {
				log.Printf("Drive %s has been unlocked", devpath)
				unlockedDisks += 1
			}
		} else {
			log.Printf("Considered drive %s, but drive is not locked", identity)
		}
	}
	return unlockedDisks, nil
}

func (unlocker DiskUnlocker) unlockDisk(d tcg.DriveIntf, password string, driveserial []byte) error {
	pin := unlocker.kdf.DeriveKey(password, driveserial)

	cs, lmeta, err := locking.Initialize(d)
	if err != nil {
		return fmt.Errorf("locking.Initialize: %v", err)
	}
	defer cs.Close()
	l, err := locking.NewSession(cs, lmeta, locking.DefaultAuthority(pin))
	if err != nil {
		return fmt.Errorf("locking.NewSession: %v", err)
	}
	defer l.Close()

	for i, r := range l.Ranges {
		if err := r.UnlockRead(); err != nil {
			log.Printf("Read unlock range %d failed: %v", i, err)
		}
		if err := r.UnlockWrite(); err != nil {
			log.Printf("Write unlock range %d failed: %v", i, err)
		}
	}

	if l.MBREnabled && !l.MBRDone {
		if err := l.SetMBRDone(true); err != nil {
			return fmt.Errorf("SetMBRDone: %v", err)
		}
	}
	return nil
}
