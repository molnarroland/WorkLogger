//go:build darwin && cgo

package platform

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework LocalAuthentication
#import <Foundation/Foundation.h>
#import <LocalAuthentication/LocalAuthentication.h>
#include <stdlib.h>
#include <string.h>

// 0 verified, 1 no way to verify (no login password), 2 failed or cancelled. *message gets the reason (free it).
static int verifyOwner(const char *reason, char **message) {
	@autoreleasepool {
		LAContext *ctx = [[[LAContext alloc] init] autorelease];
		NSError *err = nil;
		// DeviceOwnerAuthentication: Touch ID (or a paired Apple Watch) when there is one, else the login password.
		if (![ctx canEvaluatePolicy:LAPolicyDeviceOwnerAuthentication error:&err]) {
			*message = strdup(err ? err.localizedDescription.UTF8String : "unavailable");
			return err && err.code == LAErrorPasscodeNotSet ? 1 : 2;
		}
		dispatch_semaphore_t done = dispatch_semaphore_create(0);
		__block BOOL ok = NO;
		__block char *why = NULL;
		[ctx evaluatePolicy:LAPolicyDeviceOwnerAuthentication
			localizedReason:[NSString stringWithUTF8String:reason]
			reply:^(BOOL success, NSError *e) {
				ok = success;
				if (!success) why = strdup(e ? e.localizedDescription.UTF8String : "failed");
				dispatch_semaphore_signal(done);
			}];
		dispatch_semaphore_wait(done, DISPATCH_TIME_FOREVER);
		dispatch_release(done);
		*message = why;
		return ok ? 0 : 2;
	}
}
*/
import "C"

import (
	"errors"
	"unsafe"
)

// VerifyUser asks the user to prove it's them: Touch ID, or the login password. macOS shows reason as
// "“WorkLogger” is trying to <reason>." Blocks until answered, so never call it on the main thread. With no login
// password there is nothing to check against, and it passes. window is unused here.
func VerifyUser(reason string, _ unsafe.Pointer) error {
	cReason := C.CString(reason)
	defer C.free(unsafe.Pointer(cReason))
	var message *C.char
	code := C.verifyOwner(cReason, &message)
	if message != nil {
		defer C.free(unsafe.Pointer(message))
	}
	switch code {
	case 0, 1:
		return nil
	default:
		return errors.New(C.GoString(message))
	}
}
