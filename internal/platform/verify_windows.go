package platform

import (
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows Hello through WinRT's UserConsentVerifier, called without cgo: combase activates the class, and the COM
// methods are called by their vtable slots. The interop interface ties the prompt to our window, so it opens in
// front of it rather than behind.

var (
	combase                  = windows.NewLazySystemDLL("combase.dll")
	procRoInitialize         = combase.NewProc("RoInitialize")
	procRoUninitialize       = combase.NewProc("RoUninitialize")
	procRoGetActivationFact  = combase.NewProc("RoGetActivationFactory")
	procWindowsCreateString  = combase.NewProc("WindowsCreateString")
	procWindowsDeleteString  = combase.NewProc("WindowsDeleteString")
	iidUserConsentInterop    = guid{0x39e050c3, 0x4e74, 0x441a, [8]byte{0x8d, 0xc0, 0xb8, 0x11, 0x04, 0xdf, 0x94, 0x9c}}
	iidAsyncInfo             = guid{0x00000036, 0x0000, 0x0000, [8]byte{0xc0, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x46}}
	iidAsyncOperationConsent = parameterizedIID("pinterface({9fc2b0bb-e446-44e2-aa61-9cab8f636af2};" +
		"enum(Windows.Security.Credentials.UI.UserConsentVerificationResult;i4))")
)

const (
	roInitMultithreaded = 1
	rpcEChangedMode     = 0x80010106

	// Vtable slots. IUnknown: 0 QueryInterface, 2 Release; IInspectable adds 3-5.
	slotQueryInterface   = 0
	slotRelease          = 2
	slotRequestForWindow = 6 // IUserConsentVerifierInterop.RequestVerificationForWindowAsync
	slotAsyncStatus      = 7 // IAsyncInfo.get_Status
	slotAsyncCancel      = 9 // IAsyncInfo.Cancel
	slotGetResults       = 8 // IAsyncOperation<T>.GetResults

	// AsyncStatus
	asyncStarted   = 0
	asyncCompleted = 1

	// UserConsentVerificationResult
	consentVerified             = 0
	consentDeviceNotPresent     = 1
	consentNotConfiguredForUser = 2
	consentDisabledByPolicy     = 3
	consentDeviceBusy           = 4
	consentRetriesExhausted     = 5
	consentCanceled             = 6

	// The prompt is given this long before it counts as a no.
	verifyTimeout = 5 * time.Minute
)

// VerifyUser asks the user to prove it's them with Windows Hello (face, fingerprint or PIN), the prompt owned by
// window (the main window's HWND; nil works, but the prompt may open behind the app). Blocks until answered.
// It passes when Windows Hello isn't set up or is turned off by policy, or the prompt can't be shown: there is
// nothing to check against then, and failing would lock the user out of their tokens.
func VerifyUser(reason string, window unsafe.Pointer) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	switch hr := callProc(procRoInitialize, roInitMultithreaded); {
	case hr >= 0:
		defer callProc(procRoUninitialize)
	case uint32(hr) != rpcEChangedMode: // an STA thread works too
		return nil // no WinRT
	}

	class, err := newHString("Windows.Security.Credentials.UI.UserConsentVerifier")
	if err != nil {
		return nil
	}
	defer deleteHString(class)
	var interop unsafe.Pointer
	if hr := callProc(procRoGetActivationFact, class, uintptr(unsafe.Pointer(&iidUserConsentInterop)),
		uintptr(unsafe.Pointer(&interop))); hr < 0 {
		return nil // Windows without the interop interface
	}
	defer release(interop)

	message, err := newHString(reason)
	if err != nil {
		return nil
	}
	defer deleteHString(message)
	var op unsafe.Pointer
	if hr := call(interop, slotRequestForWindow, uintptr(window), message,
		uintptr(unsafe.Pointer(&iidAsyncOperationConsent)), uintptr(unsafe.Pointer(&op))); hr < 0 {
		return nil
	}
	defer release(op)

	var info unsafe.Pointer
	if hr := call(op, slotQueryInterface, uintptr(unsafe.Pointer(&iidAsyncInfo)), uintptr(unsafe.Pointer(&info))); hr < 0 {
		return fmt.Errorf("windows hello: no IAsyncInfo (0x%08x)", uint32(hr))
	}
	defer release(info)
	// Polled rather than given a completion handler: a handler would be a COM object made in Go.
	deadline := time.Now().Add(verifyTimeout)
	var status int32
	for {
		if hr := call(info, slotAsyncStatus, uintptr(unsafe.Pointer(&status))); hr < 0 {
			return fmt.Errorf("windows hello: status (0x%08x)", uint32(hr))
		}
		if status != asyncStarted {
			break
		}
		if time.Now().After(deadline) {
			call(info, slotAsyncCancel)
			return errors.New("windows hello: no answer")
		}
		time.Sleep(50 * time.Millisecond)
	}
	if status != asyncCompleted {
		return fmt.Errorf("windows hello: the request ended with status %d", status)
	}
	var result int32
	if hr := call(op, slotGetResults, uintptr(unsafe.Pointer(&result))); hr < 0 {
		return fmt.Errorf("windows hello: result (0x%08x)", uint32(hr))
	}
	switch result {
	case consentVerified, consentDeviceNotPresent, consentNotConfiguredForUser, consentDisabledByPolicy:
		return nil
	case consentCanceled:
		return errors.New("windows hello: cancelled")
	case consentRetriesExhausted:
		return errors.New("windows hello: too many tries")
	case consentDeviceBusy:
		return errors.New("windows hello: the device is busy")
	default:
		return fmt.Errorf("windows hello: result %d", result)
	}
}

type hresult int32

// call calls method slot of the COM object obj.
func call(obj unsafe.Pointer, slot int, args ...uintptr) hresult {
	vtbl := *(*unsafe.Pointer)(obj)
	fn := *(*uintptr)(unsafe.Add(vtbl, uintptr(slot)*unsafe.Sizeof(uintptr(0))))
	r, _, _ := syscall.SyscallN(fn, append([]uintptr{uintptr(obj)}, args...)...)
	return hresult(r)
}

func release(obj unsafe.Pointer) {
	if obj != nil {
		call(obj, slotRelease)
	}
}

func newHString(s string) (uintptr, error) {
	u, err := syscall.UTF16FromString(s)
	if err != nil {
		return 0, err
	}
	var h uintptr
	if hr := callProc(procWindowsCreateString, uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)-1),
		uintptr(unsafe.Pointer(&h))); hr < 0 {
		return 0, fmt.Errorf("WindowsCreateString: 0x%08x", uint32(hr))
	}
	return h, nil
}

func deleteHString(h uintptr) { callProc(procWindowsDeleteString, h) }

// callProc calls a combase function that returns an HRESULT.
func callProc(p *windows.LazyProc, args ...uintptr) hresult {
	r, _, _ := p.Call(args...)
	return hresult(r)
}
