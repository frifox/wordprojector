//go:build darwin

package shell

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

static void wpHideDockAndMenuBar(int hide) {
	dispatch_async(dispatch_get_main_queue(), ^{
		NSApplicationPresentationOptions o = NSApplicationPresentationDefault;
		if (hide) {
			o = NSApplicationPresentationHideDock | NSApplicationPresentationHideMenuBar;
		}
		[NSApp setPresentationOptions:o];
	});
}
*/
import "C"

// hideDockAndMenuBar hides the Dock and the menu bar while this program is in front (and brings them back when
// it is not, or when called with false), like a slide show in Keynote: the projection then covers the whole screen.
func hideDockAndMenuBar(hide bool) {
	if hide {
		C.wpHideDockAndMenuBar(1)
	} else {
		C.wpHideDockAndMenuBar(0)
	}
}
