package platform

import (
	"fmt"
	"testing"
)

func (g guid) String() string {
	return fmt.Sprintf("%08x-%04x-%04x-%x-%x", g.Data1, g.Data2, g.Data3, g.Data4[:2], g.Data4[2:])
}

// The Windows Hello unlock asks for IAsyncOperation<UserConsentVerificationResult> by an IID computed at run time;
// the algorithm is checked against IIDs published in the Windows SDK headers.
func TestParameterizedIID(t *testing.T) {
	for _, c := range []struct{ signature, want string }{
		{"pinterface({9fc2b0bb-e446-44e2-aa61-9cab8f636af2};b1)", "cdb5efb3-5788-509d-9be1-71ccb8a3362a"},     // IAsyncOperation<Boolean>
		{"pinterface({faa585ea-6214-4217-afda-7f46de5869b3};string)", "e2fcc7c1-3bfc-5a0b-b2b0-72e769d1cb7e"}, // IIterable<String>
		{"pinterface({913337e9-11a1-4345-a3a2-4e7f956e222d};string)", "98b9acc1-4b56-532e-ac73-03d5291cca90"}, // IVector<String>
	} {
		if got := parameterizedIID(c.signature).String(); got != c.want {
			t.Errorf("%s: got %s, want %s", c.signature, got, c.want)
		}
	}
}
