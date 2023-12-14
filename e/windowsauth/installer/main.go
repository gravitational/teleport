package main

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/ncruces/zenity"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

//go:generate go run golang.org/x/sys/windows/mkwinsyscall -output zsyscall_windows.go main.go
//sys CertAddEncodedCertificateToStore(store windows.Handle, dwCertEncodingType uint32, pbCertEncoded []byte, dwAddDisposition uint32, ppCertContext *windows.Handle) (err error) = crypt32.CertAddEncodedCertificateToStore
//sys CertOpenStore(storeProvider uintptr, msgAndCertEncodingType uint32, cryptProv uintptr, flags uint32, para string) (handle windows.Handle, err error) = crypt32.CertOpenStore
//sys FreeConsole() = kernel32.FreeConsole

//go:embed teleport.dll
var dll []byte

var (
	// CredentialProviderGUID uniquely identifies the Teleport
	// Credential Provider.
	CredentialProviderGUID = windows.GUID{
		Data1: 0xFF285315,
		Data2: 0x5335,
		Data3: 0x4F69,
		Data4: [8]byte{0xA9, 0xA2, 0x9C, 0xC5, 0xF8, 0x41, 0x9D, 0x55}}.String()

	CLSID       = `Software\Classes\CLSID\` + CredentialProviderGUID
	InprocKey   = CLSID + `\InprocServer32`
	ProgIdKey   = CLSID + `\ProgId`
	ProviderKey = `SOFTWARE\Microsoft\Windows\CurrentVersion\Authentication\Credential Providers\` + CredentialProviderGUID
	FilterKey   = `SOFTWARE\Microsoft\Windows\CurrentVersion\Authentication\Credential Provider Filters\` + CredentialProviderGUID
	LSAKey      = `SYSTEM\CurrentControlSet\Control\Lsa`
)

var (
	title  = zenity.Title("Teleport Authentication Package")
	width  = zenity.Width(300)
	height = zenity.Height(200)
)

const (
	APName  = "Teleport"
	dllPath = `C:\Windows\System32\teleport.dll`
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Launching the Teleport Windows Auth Setup GUI")
		time.Sleep(1 * time.Second)
		FreeConsole()
		ui()
		return
	}
	app := kingpin.New("teleport-windows-auth-setup.exe", "Installs Teleport Authentication Package")
	reboot := app.Flag("reboot", "reboots machine after action").Short('r').Bool()
	install := app.Command("install", "Installs Teleport Authentication Package")
	cert := install.Flag("cert", "path to Teleport CA certificate").String()
	uninstall := app.Command("uninstall", "Uninstalls Teleport Authentication Package")
	switch kingpin.MustParse(app.Parse(os.Args[1:])) {
	case install.FullCommand():
		if *cert != "" {
			app.FatalIfError(importCert(*cert), "can't import certificate form %s", *cert)
		}
		app.FatalIfError(disableNLA(), "can't disable NLA")
		app.FatalIfError(setConnectionSecurityLayer(), "can't request security layer")
		app.FatalIfError(enableRemoteFX(), "can't enable RemoteFX")
		app.FatalIfError(copyDLL(), "can't install Teleport Authentication Package")
		app.FatalIfError(registerDLL(), "can't register Teleport Authentication Package")
		fmt.Println("Teleport Authentication Package installed")
	case uninstall.FullCommand():
		app.FatalIfError(unregisterDLL(), "can't unregister dll")
		app.FatalIfError(deleteOldDLL(), "can't delete dll")
		fmt.Println("Teleport Authentication Package uninstalled")
	}
	if *reboot {
		app.FatalIfError(rebootWindows(), "can't reboot Windows")
	}
}

var certificateFilter = zenity.FileFilter{
	Name:     "Certificate files",
	Patterns: []string{"*.cer"},
}

func ui() {
	if _, err := os.Stat(dllPath); err == nil {
		err := zenity.Question("Teleport Authentication Package is already installed.\n\nWhat would you like to do?",
			title, width, height, zenity.OKLabel("Update"), zenity.ExtraButton("Uninstall"))
		if err == zenity.ErrExtraButton {
			if err := unregisterDLL(); err != nil {
				zenity.Error(fmt.Sprintf("Can't unregister Teleport Authentication Package: %s", err), title, width, height)
				return
			}
			if err := deleteOldDLL(); err != nil {
				zenity.Error(fmt.Sprintf("Can't delete Teleport Authentication Package: %s", err), title, width, height)
				return
			}
			if err := zenity.Question("Teleport Authentication Package uninstalled successfully.\nRestart now?",
				title, width, height); err == nil {
				if err := rebootWindows(); err != nil {
					zenity.Error(fmt.Sprintf("Can't reboot Windows: %s", err), title, width, height)
				}
			}
		} else if err == nil {
			if file, err := zenity.SelectFile(
				zenity.Filename(""),
				zenity.FileFilters{certificateFilter},
				zenity.Title("Select Teleport CA Certificate, or cancel to use the existing certificate")); err == nil {
				if err := importCert(file); err != nil {
					zenity.Error(fmt.Sprintf("Can't import certificate: %s", err), title, width, height)
					return
				}
			}

			if err := disableNLA(); err != nil {
				zenity.Error(fmt.Sprintf("Can't disable NLA: %s", err), title, width, height)
				return
			}
			if err := setConnectionSecurityLayer(); err != nil {
				zenity.Error(fmt.Sprintf("Can't request security layer: %s", err), title, width, height)
				return
			}
			if err := enableRemoteFX(); err != nil {
				zenity.Error(fmt.Sprintf("Can't enable RemoteFX: %s", err), title, width, height)
				return
			}
			if err := copyDLL(); err != nil {
				zenity.Error(fmt.Sprintf("Can't update Teleport Authentication Package: %s", err), title, width, height)
				return
			}
			if err := registerDLL(); err != nil {
				zenity.Error(fmt.Sprintf("Can't register Teleport Authentication Package: %s", err), title, width, height)
				return
			}
			if err := zenity.Question("Teleport Authentication Package updated successfully.\n"+
				"Restart is required.\n"+
				"Restart now?", title, width, height); err == nil {
				if err := rebootWindows(); err != nil {
					zenity.Error(fmt.Sprintf("Can't reboot Windows: %s", err), title, width, height)
				}
			}
		}
		return
	}
	if err := zenity.Question("To connect to this machine using Teleport this installer will:\n"+
		"1. Install Teleport Authentication Package\n"+
		"2. Import Teleport CA Certificate\n"+
		"3. Disable NLA\n\n"+
		"All these steps are required.\n"+
		"Proceed?", title, width, height); err != nil {
		return
	}
	file, err := zenity.SelectFile(
		zenity.Filename(""),
		zenity.FileFilters{certificateFilter},
		zenity.Title("Select Teleport CA certificate"))
	if err != nil {
		return
	}
	if err := importCert(file); err != nil {
		zenity.Error(fmt.Sprintf("Can't import certificate: %s", err), title, width, height)
		return
	}
	if err := disableNLA(); err != nil {
		zenity.Error(fmt.Sprintf("Can't disable NLA: %s", err), title, width, height)
		return
	}
	if err := setConnectionSecurityLayer(); err != nil {
		zenity.Error(fmt.Sprintf("Can't request security layer: %s", err), title, width, height)
		return
	}
	if err := enableRemoteFX(); err != nil {
		zenity.Error(fmt.Sprintf("Can't enable RemoteFX: %s", err), title, width, height)
		return
	}
	if err := copyDLL(); err != nil {
		zenity.Error(fmt.Sprintf("Can't install Teleport Authentication Package: %s", err), title, width, height)
		return
	}
	if err := registerDLL(); err != nil {
		zenity.Error(fmt.Sprintf("Can't register Teleport Authentication Package: %s", err), title, width, height)
		return
	}
	if err := zenity.Question("Teleport Authentication Package installed successfully.\n"+
		"Restart is required.\n"+
		"Restart now?", title, width, height); err == nil {
		if err := exec.Command("cmd", "/C", "shutdown", "/r", "/t", "0", "/f").Run(); err != nil {
			zenity.Error(fmt.Sprintf("Can't reboot Windows: %s", err), title, width, height)
		}
	}
}

func rebootWindows() error {
	return exec.Command("cmd", "/C", "shutdown", "/r", "/t", "0", "/f").Run()
}

func copyDLL() error {
	err := deleteOldDLL()
	if err != nil {
		return err
	}
	if err := os.WriteFile(dllPath, dll, 0444); err != nil {
		return fmt.Errorf("can't copy DLL: %w", err)
	}
	return nil
}

func deleteOldDLL() error {
	if _, err := os.Stat(dllPath); err == nil {
		newName := fmt.Sprintf("C:\\Windows\\System32\\teleport_old_%d.dll", time.Now().UnixMilli())
		if err := os.Rename(dllPath, newName); err != nil {
			return fmt.Errorf("can't move DLL: %w", err)
		}
		uname, err := windows.UTF16PtrFromString(newName)
		if err != nil {
			return err
		}
		// this will delete file on reboot
		// see https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-movefileexw
		windows.MoveFileEx(uname, nil, windows.MOVEFILE_DELAY_UNTIL_REBOOT)
	}
	return nil
}

func importCert(file string) error {
	der, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("can't read file %s, error: %w", file, err)
	}
	hstore, err := CertOpenStore(windows.CERT_STORE_PROV_SYSTEM_A, 0, 0, windows.CERT_SYSTEM_STORE_LOCAL_MACHINE, "ROOT")
	if err != nil {
		return fmt.Errorf("can't open certificate store: %w", err)
	}
	defer windows.CertCloseStore(hstore, 0)
	if err := CertAddEncodedCertificateToStore(hstore, windows.X509_ASN_ENCODING|windows.PKCS_7_ASN_ENCODING, der, windows.CERT_STORE_ADD_REPLACE_EXISTING, nil); err != nil {
		return fmt.Errorf("can't add certificate to store: %w", err)
	}
	return nil
}

func enableRemoteFX() error {
	key := `SOFTWARE\Policies\Microsoft\Windows NT\Terminal Services`
	servicesKey, err := registry.OpenKey(registry.LOCAL_MACHINE, key, registry.ALL_ACCESS)
	if err != nil {
		return fmt.Errorf("opening key %s: %w", key, err)
	}
	defer servicesKey.Close()
	if err := servicesKey.SetDWordValue("ColorDepth", 5); err != nil {
		return fmt.Errorf("setting ColorDepth: %w", err)
	}
	if err := servicesKey.SetDWordValue("fEnableVirtualizedGraphics", 1); err != nil {
		return fmt.Errorf("setting fEnableVirtualizedGraphics: %w", err)
	}
	return nil
}

func disableNLA() error {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\Terminal Server\WinStations\RDP-Tcp`, registry.ALL_ACCESS)
	if err != nil {
		return fmt.Errorf("error opening RDP-Tcp key: %w", err)
	}
	defer key.Close()
	if err := key.SetDWordValue("UserAuthentication", 0); err != nil {
		return fmt.Errorf("can't set UserAuthentication value: %w", err)
	}
	return nil
}

// setConnectionSecurityLayer sets  connection security layer to Negotiate (server and client will choose between RDP and TLS).
// Teleport requires secure connection (TLS) but we don't want to prevent access from older clients.
//
// See https://learn.microsoft.com/en-us/windows-hardware/customize/desktop/unattend/microsoft-windows-terminalservices-rdp-winstationextensions-securitylayer
func setConnectionSecurityLayer() error {
	key := `SOFTWARE\Policies\Microsoft\Windows NT\Terminal Services`
	servicesKey, err := registry.OpenKey(registry.LOCAL_MACHINE, key, registry.ALL_ACCESS)
	if err != nil {
		return fmt.Errorf("opening key %s: %w", key, err)
	}
	defer servicesKey.Close()
	if err := servicesKey.SetDWordValue("SecurityLayer", 1); err != nil {
		return fmt.Errorf("setting security layer: %w", err)
	}
	return nil
}

func registerDLL() error {
	key, _, err := registry.CreateKey(registry.LOCAL_MACHINE, CLSID, registry.ALL_ACCESS)
	if err != nil {
		return fmt.Errorf("error creating key: %w", err)
	}
	defer key.Close()
	if err := key.SetStringValue("", APName); err != nil {
		return fmt.Errorf("error setting def value: %w", err)
	}
	subkey, _, err := registry.CreateKey(key, "InprocServer32", registry.ALL_ACCESS)
	if err != nil {
		return fmt.Errorf("error creating subkey: %w", err)
	}
	defer subkey.Close()
	if err := subkey.SetStringValue("", dllPath); err != nil {
		return fmt.Errorf("error setting subkey def value: %w", err)
	}
	if err := subkey.SetStringValue("ThreadingModel", "Both"); err != nil {
		return fmt.Errorf("error setting subkey ThreadingModel value: %w", err)
	}
	subkey, _, err = registry.CreateKey(key, "ProgId", registry.ALL_ACCESS)
	if err != nil {
		return fmt.Errorf("error creating subkey: %w", err)
	}
	defer subkey.Close()
	if err := subkey.SetStringValue("", APName); err != nil {
		return fmt.Errorf("error setting subkey def value: %w", err)
	}
	key, _, err = registry.CreateKey(registry.LOCAL_MACHINE, ProviderKey, registry.ALL_ACCESS)
	if err != nil {
		return fmt.Errorf("error creating credential provider key: %w", err)
	}
	defer key.Close()
	if err := key.SetStringValue("", APName); err != nil {
		return fmt.Errorf("error setting def value: %w", err)
	}
	key, _, err = registry.CreateKey(registry.LOCAL_MACHINE, FilterKey, registry.ALL_ACCESS)
	if err != nil {
		return fmt.Errorf("error creating credential provider key: %w", err)
	}
	defer key.Close()
	if err := key.SetStringValue("", APName); err != nil {
		return fmt.Errorf("error setting def value: %w", err)
	}
	key, err = registry.OpenKey(registry.LOCAL_MACHINE, LSAKey, registry.ALL_ACCESS)
	if err != nil {
		return fmt.Errorf("error opening LSA key: %w", err)
	}
	defer key.Close()
	packages, _, err := key.GetStringsValue("Authentication Packages")
	if err != nil {
		return fmt.Errorf("error getting Authentication Packages value: %w", err)
	}
	for _, p := range packages {
		if p == "teleport" {
			return nil
		}
	}
	packages = append(packages, "teleport")
	if err := key.SetStringsValue("Authentication Packages", packages); err != nil {
		return fmt.Errorf("error setting def value: %w", err)
	}
	return nil
}

func unregisterDLL() error {
	if err := registry.DeleteKey(registry.LOCAL_MACHINE, InprocKey); err != nil {
		return fmt.Errorf("can't delete InprocServer32 key: %w", err)
	}
	if err := registry.DeleteKey(registry.LOCAL_MACHINE, ProgIdKey); err != nil {
		return fmt.Errorf("can't delete ProgId key: %w", err)
	}
	if err := registry.DeleteKey(registry.LOCAL_MACHINE, CLSID); err != nil {
		return fmt.Errorf("can't delete CLSID key: %w", err)
	}
	if err := registry.DeleteKey(registry.LOCAL_MACHINE, ProviderKey); err != nil {
		return fmt.Errorf("can't delete credential provider key: %w", err)
	}
	if err := registry.DeleteKey(registry.LOCAL_MACHINE, FilterKey); err != nil {
		return fmt.Errorf("can't delete credential provider filter key: %w", err)
	}
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, LSAKey, registry.ALL_ACCESS)
	if err != nil {
		return fmt.Errorf("error opening LSA key: %w", err)
	}
	defer key.Close()
	packages, _, err := key.GetStringsValue("Authentication Packages")
	if err != nil {
		return fmt.Errorf("error getting Authentication Packages value: %w", err)
	}
	var filtered []string
	for _, p := range packages {
		if p != "teleport" {
			filtered = append(filtered, p)
		}
	}
	if err := key.SetStringsValue("Authentication Packages", filtered); err != nil {
		return fmt.Errorf("error setting def value: %w", err)
	}
	return nil
}
