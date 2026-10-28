//go:build darwin

package shell

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework QuartzCore -framework UniformTypeIdentifiers
#import <Cocoa/Cocoa.h>
#import <QuartzCore/QuartzCore.h>
#import <UniformTypeIdentifiers/UniformTypeIdentifiers.h>
#include <stdlib.h>

// told to Go (see wpProjectorEvent in presentation_darwin.go) when the native side has done what it was asked
extern void wpProjectorEvent(int, int);

static void wpHideDockAndMenuBar(int hide) {
	dispatch_async(dispatch_get_main_queue(), ^{
		NSApplicationPresentationOptions o = NSApplicationPresentationDefault;
		if (hide) {
			o = NSApplicationPresentationHideDock | NSApplicationPresentationHideMenuBar;
		}
		[NSApp setPresentationOptions:o];
	});
}
static void wpActivate() {
	dispatch_async(dispatch_get_main_queue(), ^{
		[NSApp activateIgnoringOtherApps:YES];
	});
}

// A display window is never hidden: it stays ordered in, fully transparent and deaf to the mouse, so its page keeps
// running (a hidden web view gets no animation frames and cannot say that it has drawn). Showing it is changing its
// alpha, one frame. Kinds told to Go: 1 = ordered in, invisible; 2 = visible, committed to the screen; 3 = invisible,
// committed.
static void wpProjectorSetVisible(void *win, int disp, int visible, int first) {
	dispatch_async(dispatch_get_main_queue(), ^{
		NSWindow *w = (__bridge NSWindow *)win;
		w.ignoresMouseEvents = visible ? NO : YES;
		if (first) {
			w.collectionBehavior |= NSWindowCollectionBehaviorIgnoresCycle;
		}
		// Edge to edge, nothing to see through: no shadow (it draws a thin outline and bleeds over a neighbouring screen), an opaque
		// black window, and the web view exactly as big as the content area (the library makes it a point larger)
		w.hasShadow = NO;
		w.opaque = YES;
		w.backgroundColor = [NSColor blackColor];
		NSView *cv = w.contentView;
		if (cv != nil) {
			for (NSView *v in cv.subviews) v.frame = cv.bounds;
		}
		[CATransaction begin];
		[CATransaction setCompletionBlock:^{
			wpProjectorEvent(disp, first ? 1 : (visible ? 2 : 3));
		}];
		w.alphaValue = visible ? 1.0 : 0.0;
		[w orderFrontRegardless];
		[CATransaction commit];
	});
}

// wpOpenPanel shows a file or folder panel (a sheet on the window when there is one) and waits for the answer: the paths
// chosen, one per line, or NULL when it was cancelled. The files are limited by type only; Wails' own panel also
// installs a delegate, which the panel asks about every file of every folder it shows (slow in big folders).
static char *wpOpenPanel(void *win, const char *title, const char *exts, int multiple, int dirs) {
	__block NSString *result = nil;
	dispatch_semaphore_t done = dispatch_semaphore_create(0);
	NSString *t = title ? [NSString stringWithUTF8String:title] : @"";
	NSString *e = exts ? [NSString stringWithUTF8String:exts] : @"";
	dispatch_async(dispatch_get_main_queue(), ^{
		NSOpenPanel *panel = [NSOpenPanel openPanel];
		panel.message = t;
		panel.canChooseFiles = !dirs;
		panel.canChooseDirectories = dirs ? YES : NO;
		panel.canCreateDirectories = dirs ? YES : NO;
		panel.allowsMultipleSelection = multiple ? YES : NO;
		if (!dirs && e.length > 0) {
			NSMutableArray *types = [NSMutableArray array];
			for (NSString *x in [e componentsSeparatedByString:@";"]) {
				UTType *u = [UTType typeWithFilenameExtension:x];
				if (u) [types addObject:u];
			}
			if (types.count > 0) panel.allowedContentTypes = types;
		}
		void (^finish)(NSInteger) = ^(NSInteger r) {
			if (r == NSModalResponseOK) {
				NSMutableArray *paths = [NSMutableArray array];
				for (NSURL *u in panel.URLs) [paths addObject:u.path];
				result = [paths componentsJoinedByString:@"\n"];
			}
			dispatch_semaphore_signal(done);
		};
		if (win != NULL) {
			[panel beginSheetModalForWindow:(__bridge NSWindow *)win completionHandler:finish];
		} else {
			[NSApp activateIgnoringOtherApps:YES];
			[panel beginWithCompletionHandler:finish];
		}
	});
	dispatch_semaphore_wait(done, DISPATCH_TIME_FOREVER);
	return result ? strdup([result UTF8String]) : NULL;
}

*/
import "C"

import (
	"strings"
	"unsafe"
)

// projectorEvents receives what the native side tells about a display window: disp and kind (see wpProjectorSetVisible).
var projectorEvents func(disp, kind int)

//export wpProjectorEvent
func wpProjectorEvent(disp, kind C.int) {
	if f := projectorEvents; f != nil {
		f(int(disp), int(kind))
	}
}

// haveWarmWindows: a display window stays ordered in and is only made visible or invisible (see setProjectorVisible).
const haveWarmWindows = true

// setProjectorVisible makes the display window visible or invisible (first: the window is being set up, invisible).
// It answers through projectorEvents once the change is on the screen.
func setProjectorVisible(win unsafe.Pointer, disp int, visible, first bool) {
	v, f := 0, 0
	if visible {
		v = 1
	}
	if first {
		f = 1
	}
	C.wpProjectorSetVisible(win, C.int(disp), C.int(v), C.int(f))
}

// nativeOpen asks for files (or a folder) with a panel of our own. cancelled is true when the user closed it.
func nativeOpen(win unsafe.Pointer, title string, exts []string, multiple, dirs bool) (paths []string, cancelled bool) {
	ct, ce := C.CString(title), C.CString(strings.Join(exts, ";"))
	defer C.free(unsafe.Pointer(ct))
	defer C.free(unsafe.Pointer(ce))
	m, d := 0, 0
	if multiple {
		m = 1
	}
	if dirs {
		d = 1
	}
	out := C.wpOpenPanel(win, ct, ce, C.int(m), C.int(d))
	if out == nil {
		return nil, true
	}
	defer C.free(unsafe.Pointer(out))
	return strings.Split(C.GoString(out), "\n"), false
}

const haveNativeOpen = true

// bringToFront makes this program the active one. A background program (no window, no Dock icon) is never in front, and
// a file dialog it opens would appear behind the other windows.
func bringToFront() { C.wpActivate() }

// hideDockAndMenuBar hides the Dock and the menu bar while this program is in front (and brings them back when
// it is not, or when called with false), like a slide show in Keynote: the projection then covers the whole screen.
func hideDockAndMenuBar(hide bool) {
	if hide {
		C.wpHideDockAndMenuBar(1)
	} else {
		C.wpHideDockAndMenuBar(0)
	}
}
