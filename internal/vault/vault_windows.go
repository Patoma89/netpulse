//go:build windows

package vault

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	targetPrefix = "NetPulse:"

	credTypeGeneric         uint32 = 1
	credPersistLocalMachine uint32 = 2
)

// ErrNotFound is returned when a requested credential does not exist in Windows Credential Manager.
var ErrNotFound = errors.New("credential not found")

var (
	advapi32   = windows.NewLazySystemDLL("advapi32.dll")
	credWriteW = advapi32.NewProc("CredWriteW")
	credReadW  = advapi32.NewProc("CredReadW")
	credDelete = advapi32.NewProc("CredDeleteW")
	credFree   = advapi32.NewProc("CredFree")
)

// credentialW matches the Win32 CREDENTIALW struct layout in wincred.h.
type credentialW struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWritten        windows.Filetime
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

// Vault wraps Windows Credential Manager using pure-Go advapi32.dll syscalls (no CGO).
type Vault struct{}

// New creates a new Windows Credential Manager Vault instance.
func New() *Vault {
	return &Vault{}
}

func targetName(key string) (string, error) {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return "", errors.New("vault key cannot be empty")
	}
	if strings.HasPrefix(trimmed, targetPrefix) {
		return trimmed, nil
	}
	return targetPrefix + trimmed, nil
}

// HealthCheck verifies that advapi32.dll credential procedures resolve cleanly on this host.
func (v *Vault) HealthCheck() error {
	for _, proc := range []*windows.LazyProc{credWriteW, credReadW, credDelete, credFree} {
		if err := proc.Find(); err != nil {
			return fmt.Errorf("credential manager procedure %s unavailable: %w", proc.Name, err)
		}
	}
	return nil
}

// SetSecret stores a username/password pair in the local Windows Credential Manager.
func (v *Vault) SetSecret(key, username, password string) error {
	target, err := targetName(key)
	if err != nil {
		return err
	}

	targetPtr, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	userPtr, err := windows.UTF16PtrFromString(username)
	if err != nil {
		return err
	}
	commentPtr, err := windows.UTF16PtrFromString("NetPulse Local Vault")
	if err != nil {
		return err
	}

	blob := encodeUTF16LE(password)
	var blobPtr *byte
	if len(blob) > 0 {
		blobPtr = &blob[0]
	}

	cred := credentialW{
		Type:               credTypeGeneric,
		TargetName:         targetPtr,
		Comment:            commentPtr,
		CredentialBlobSize: uint32(len(blob)),
		CredentialBlob:     blobPtr,
		Persist:            credPersistLocalMachine,
		UserName:           userPtr,
	}

	r1, _, callErr := credWriteW.Call(uintptr(unsafe.Pointer(&cred)), 0)
	if r1 == 0 {
		return fmt.Errorf("CredWriteW failed: %w", callErr)
	}
	return nil
}

// GetSecret retrieves a stored username and password from Windows Credential Manager.
func (v *Vault) GetSecret(key string) (string, string, error) {
	target, err := targetName(key)
	if err != nil {
		return "", "", err
	}
	targetPtr, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return "", "", err
	}

	var credPtr *credentialW
	r1, _, callErr := credReadW.Call(
		uintptr(unsafe.Pointer(targetPtr)),
		uintptr(credTypeGeneric),
		0,
		uintptr(unsafe.Pointer(&credPtr)),
	)
	if r1 == 0 {
		if errors.Is(callErr, windows.ERROR_NOT_FOUND) {
			return "", "", ErrNotFound
		}
		return "", "", fmt.Errorf("CredReadW failed: %w", callErr)
	}
	defer credFree.Call(uintptr(unsafe.Pointer(credPtr)))

	var username string
	if credPtr.UserName != nil {
		username = windows.UTF16PtrToString(credPtr.UserName)
	}

	var password string
	if credPtr.CredentialBlob != nil && credPtr.CredentialBlobSize > 0 {
		raw := unsafe.Slice(credPtr.CredentialBlob, int(credPtr.CredentialBlobSize))
		password = decodeUTF16LE(raw)
	}

	return username, password, nil
}

// DeleteSecret removes a stored credential from Windows Credential Manager.
func (v *Vault) DeleteSecret(key string) error {
	target, err := targetName(key)
	if err != nil {
		return err
	}
	targetPtr, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}

	r1, _, callErr := credDelete.Call(
		uintptr(unsafe.Pointer(targetPtr)),
		uintptr(credTypeGeneric),
		0,
	)
	if r1 == 0 {
		if errors.Is(callErr, windows.ERROR_NOT_FOUND) {
			return ErrNotFound
		}
		return fmt.Errorf("CredDeleteW failed: %w", callErr)
	}
	return nil
}

// HasSecret reports whether a credential exists for the given key.
func (v *Vault) HasSecret(key string) bool {
	_, _, err := v.GetSecret(key)
	return err == nil
}

func encodeUTF16LE(s string) []byte {
	if s == "" {
		return nil
	}
	u16 := utf16.Encode([]rune(s))
	buf := make([]byte, len(u16)*2)
	for i, v := range u16 {
		binary.LittleEndian.PutUint16(buf[i*2:], v)
	}
	return buf
}

func decodeUTF16LE(b []byte) string {
	if len(b) < 2 {
		return ""
	}
	u16 := make([]uint16, len(b)/2)
	for i := range u16 {
		u16[i] = binary.LittleEndian.Uint16(b[i*2:])
	}
	return string(utf16.Decode(u16))
}
