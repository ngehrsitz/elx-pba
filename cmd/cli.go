package cmd

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/elastx/elx-pba/cmd/internal/authentication"
	"github.com/elastx/elx-pba/cmd/internal/keyderiviation"

	"github.com/u-root/u-root/pkg/libinit"
	"github.com/u-root/u-root/pkg/mount"
	"github.com/u-root/u-root/pkg/ulog"
	"golang.org/x/term"
)

//go:embed logo.txt
var logo []byte

var (
	Version = "(devel)"
	GitHash = "(no hash)"
)

type CLI struct {
	unlocker DiskUnlocker
}

func NewCLI(auth authentication.Authenticator, kdf keyderiviation.KeyDerivationFunction) *CLI {
	return &CLI{unlocker: DiskUnlocker{auth, kdf}}
}

func (cli *CLI) Start() {
	fmt.Printf("\n")
	fmt.Println(string(logo))
	fmt.Printf("Welcome to Elastx PBA!\nSource: %s\nGit Info: %s\n\n", Version, GitHash)
	log.SetPrefix("elx-pba: ")

	if _, err := mount.Mount("proc", "/proc", "proc", "", 0); err != nil {
		log.Fatalf("Mount(proc): %v", err)
	}
	if _, err := mount.Mount("sysfs", "/sys", "sysfs", "", 0); err != nil {
		log.Fatalf("Mount(sysfs): %v", err)
	}
	if _, err := mount.Mount("efivarfs", "/sys/firmware/efi/efivars", "efivarfs", "", 0); err != nil {
		log.Fatalf("Mount(efivars): %v", err)
	}

	log.Printf("Starting system...")

	if err := ulog.KernelLog.SetConsoleLogLevel(ulog.KLogNotice); err != nil {
		log.Printf("Could not set log level KLogNotice: %v", err)
	}

	libinit.SetEnv()
	libinit.CreateRootfs()
	libinit.NetInit()

	defer func() {
		log.Printf("Starting emergency shell...")
		for {
			Execute("/bbin/elvish")
		}
	}()
	unlockedDisks, unlockedErr := cli.unlocker.UnlockDisks()
	if unlockedErr != nil {
		log.Printf("Failed to unlock disks: %v", unlockedErr)
		return
	}
	if unlockedDisks < 1 {
		log.Printf("No drives changed state to unlocked, starting shell for troubleshooting")
		return
	}

	fmt.Println()
	if waitForEnter("Starting OS in 3 seconds, press Enter to start shell instead: ", 3) {
		return
	}

	// reboot for now as 'boot' would mount filesystems and therefore mess up hibernation :-(
	// note that ext3 or ext4 will replay its journal even when mounted read-only if the filesystem is dirty
	Execute("/bbin/shutdown", "reboot")
}

func waitForEnter(prompt string, seconds int) bool {
	f, consoleErr := os.OpenFile("/dev/console", os.O_RDWR, 0)
	if consoleErr != nil {
		log.Printf("ERROR: Open /dev/console failed: %v", consoleErr)
		return false
	}
	defer f.Close()

	oldState, rawErr := term.MakeRaw(int(f.Fd()))
	if rawErr != nil {
		log.Printf("ERROR: MakeRaw failed for Fd %d: %v", f.Fd(), rawErr)
		return false
	}
	defer term.Restore(int(f.Fd()), oldState)

	nonblockErr := syscall.SetNonblock(int(f.Fd()), true)
	if nonblockErr != nil {
		log.Printf("ERROR: SetNonblock failed for Fd %d: %v", f.Fd(), nonblockErr)
		return false
	}

	newTerm := term.NewTerminal(f, prompt)
	for i := 0; i < seconds*2; i++ {
		if i > 0 {
			fmt.Print(".")
		}
		deadlineErr := f.SetDeadline(time.Now().Add(500 * time.Millisecond))
		if deadlineErr != nil {
			log.Printf("ERROR: SetDeadline failed for Fd %d: %v", f.Fd(), deadlineErr)
			return false
		}
		_, readErr := newTerm.ReadLine()
		if readErr == nil {
			return true
		}
	}

	// nobody pressed enter (need \r to reset start of line)
	fmt.Println("\r")
	return false
}

func Execute(name string, args ...string) {
	environ := append(os.Environ(), "USER=root")
	environ = append(environ, "HOME=/root")
	environ = append(environ, "TZ=UTC")

	cmd := exec.Command(name, args...)
	cmd.Dir = "/"
	cmd.Env = environ
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setctty = true
	cmd.SysProcAttr.Setsid = true
	if err := cmd.Run(); err != nil {
		log.Printf("Failed to execute: %v", err)
	}
}
