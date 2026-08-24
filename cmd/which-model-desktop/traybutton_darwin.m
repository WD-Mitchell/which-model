//go:build darwin && !ios

// Repairs left-click delivery for the menu-bar icon on modern macOS.
//
// THE BUG (upstream, in Wails v3 — identical in beta.9 and beta.10). On a real
// left-click, macOS DOES invoke the status item's action: Wails'
// statusItemClicked: runs. But that handler forwards [NSApp currentEvent].type
// to Go, and at dispatch time the current event is NOT the click — measured on
// macOS 27, it is type 5 (NSEventTypeMouseMoved). Wails' Go switch
// (systemtray_darwin.go processClick) only handles LeftMouseDown (1) and
// RightMouseDown (3), so the click is dropped silently. The user sees the
// system's click highlight and nothing else.
//
// Right-click never depended on that switch: Wails' own NSEvent local monitor
// sees the right-mouse-DOWN (measured: type=3 reaches the app), sets
// statusItem.menu, and lets AppKit run native menu tracking. That is why the
// menu always worked while left-click never did.
//
// THE FIX is the swizzle below: replace statusItemClicked: with a shim that
// treats every non-right invocation as a left click and calls straight into Go,
// forwarding right ones to the original implementation. Two further routes are
// armed as belt-and-braces for other macOS versions — the button's own
// target/action (set on mouse-UP; the supported idiom) and a local event
// monitor keyed on the status-bar window class. All three funnel into one Go
// handler behind a debounce, so at most one toggle per physical click.
//
// Paths that were tried and measured DEAD on macOS 27, kept out of the design:
//   - Wails' arming alone (the shipped behaviour): dropped in the type switch.
//   - Walking NSApp.windows for the button: the status window is system-hosted
//     and never appears there.
//   - A monitor watching for left mouse-down/up on the status window: left
//     events are consumed before monitors run (only right-downs appear).
//   - Synthetic CGEvent clicks to test from outside: TCC denies posting.

#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>

// Implemented in Go (traybutton_darwin.go).
extern void wmTrayButtonClicked(void);

// Set from Go when WM_TRAYDEBUG=1: logs what each path sees, so a miss can be
// diagnosed from a single click.
static int wmTrayDebug = 0;

void wmSetTrayDebug(int on) { wmTrayDebug = on; }

// Retained for the process lifetime. target/action references are unretained,
// and the monitor is never removed, so the app owns both until exit.
static id wmTrayMonitor = nil;

// Original IMP of Wails' StatusItemController statusItemClicked:, kept so the
// shim can forward right-clicks to it unchanged.
static void (*wmOrigStatusItemClicked)(id, SEL, id) = NULL;
static int wmSwizzled = 0;

// wmStatusItemClickedShim replaces Wails' statusItemClicked:. Wails' original
// forwards [NSApp currentEvent].type into a Go switch that only handles
// LeftMouseDown (1) and RightMouseDown (3) — systemtray_darwin.go processClick.
// If the OS invokes the item-level action on the mouse-UP (type 2), that switch
// drops the click silently, which matches the observed total silence on left
// click. The shim claims every non-right event for the popover toggle and
// forwards right ones to the original so the menu path is untouched.
static void wmStatusItemClickedShim(id self, SEL _cmd, id sender) {
	NSEvent *event = [NSApp currentEvent];
	long type = event == nil ? -1 : (long)event.type;
	if (wmTrayDebug) {
		NSLog(@"wm-traydebug: statusItemClicked shim, currentEvent type=%ld", type);
	}
	if (type == NSEventTypeRightMouseDown || type == NSEventTypeRightMouseUp) {
		if (wmOrigStatusItemClicked != NULL) {
			wmOrigStatusItemClicked(self, _cmd, sender);
		}
		return;
	}
	wmTrayButtonClicked();
}

// wmSwizzleStatusItemAction hooks Wails' controller class the first time a
// status item with a target is seen. Class-wide, but the app has exactly one
// tray. Safe if the target is absent or of an unexpected class: no method, no
// swizzle.
static void wmSwizzleStatusItemAction(NSStatusItem *item) {
	if (wmSwizzled) {
		return;
	}
#pragma clang diagnostic push
#pragma clang diagnostic ignored "-Wdeprecated-declarations"
	id controller = [item target]; // deprecated getter; Wails itself relies on it
#pragma clang diagnostic pop
	if (controller == nil) {
		return;
	}
	Method m = class_getInstanceMethod([controller class], @selector(statusItemClicked:));
	if (m == NULL) {
		return;
	}
	wmOrigStatusItemClicked = (void (*)(id, SEL, id))method_getImplementation(m);
	method_setImplementation(m, (IMP)wmStatusItemClickedShim);
	wmSwizzled = 1;
	if (wmTrayDebug) {
		NSLog(@"wm-traydebug: swizzled %@ statusItemClicked:", NSStringFromClass([controller class]));
	}
}

@interface WMTrayTarget : NSObject
- (void)wmClicked:(id)sender;
@end

@implementation WMTrayTarget
- (void)wmClicked:(id)sender {
	// The button is armed for both mouse-downs, but right-click belongs to the
	// native menu path — claiming it here would open the popover as well as the
	// menu. currentEvent is the mouse-down being dispatched right now.
	NSEvent *event = [NSApp currentEvent];
	if (wmTrayDebug) {
		NSLog(@"wm-traydebug: button action fired, currentEvent type=%ld",
			event == nil ? -1L : (long)event.type);
	}
	if (event != nil && (event.type == NSEventTypeRightMouseDown ||
	                     event.type == NSEventTypeRightMouseUp)) {
		return;
	}
	wmTrayButtonClicked();
}
@end

static WMTrayTarget *wmTrayTarget = nil;

// The button most recently rewired, retained so wmTestClickStatusButton can
// drive it. Diagnostic only.
static NSStatusBarButton *wmLastButton = nil;

static void wmEnsureTarget(void) {
	if (wmTrayTarget == nil) {
		wmTrayTarget = [[WMTrayTarget alloc] init];
	}
}

// wmRewireStatusButtons points every status item's button at our target.
// Returns the number rewired; 0 means the status item does not exist yet (the
// caller retries) or the private key is gone (the monitor below still applies).
int wmRewireStatusButtons(void) {
	wmEnsureTarget();

	NSArray *items = nil;
	@try {
		// Private, read-only, and guarded: if the key ever disappears this
		// raises, we swallow it, and the event monitor remains as the fallback.
		items = [[NSStatusBar systemStatusBar] valueForKey:@"_items"];
	} @catch (NSException *e) {
		if (wmTrayDebug) {
			NSLog(@"wm-traydebug: NSStatusBar _items unavailable: %@", e.reason);
		}
		return 0;
	}

	int rewired = 0;
	for (id item in items) {
		if (![item isKindOfClass:[NSStatusItem class]]) {
			continue;
		}
		NSStatusBarButton *button = [(NSStatusItem *)item button];
		if (button == nil) {
			continue;
		}
		[button setTarget:wmTrayTarget];
		[button setAction:@selector(wmClicked:)];
		// Mouse-UP, not mouse-DOWN. Wails arms mouse-DOWN, and measurement shows
		// a left mouse-DOWN on a status item is never delivered to this process
		// at all (a local monitor watching both buttons logged right-downs and
		// zero left-downs): AppKit runs a nested tracking loop for the left
		// button that consumes the down event. Mouse-up survives that loop,
		// which is why every working menu-bar app uses
		// sendAction(on: [.leftMouseUp, .rightMouseUp]).
		[button sendActionOn:(NSEventMaskLeftMouseUp | NSEventMaskRightMouseUp)];
		wmLastButton = [button retain];
		// Third delivery path: the item-level action (see the shim above).
		wmSwizzleStatusItemAction((NSStatusItem *)item);
		rewired++;

		if (wmTrayDebug) {
			NSLog(@"wm-traydebug: rewired button=%@ window=%@ contentView=%@",
				NSStringFromClass([button class]),
				button.window == nil ? @"(nil)" : NSStringFromClass([button.window class]),
				button.window.contentView == nil ? @"(nil)"
					: NSStringFromClass([button.window.contentView class]));
		}
	}
	return rewired;
}

// wmViewTreeHasStatusButton reports whether view or any descendant is an
// NSStatusBarButton. The status window's contentView is not always the button
// itself — it can be a wrapper — so an isKindOfClass: test on contentView alone
// misses the click.
static BOOL wmViewTreeHasStatusButton(NSView *view) {
	if (view == nil) {
		return NO;
	}
	if ([view isKindOfClass:[NSStatusBarButton class]]) {
		return YES;
	}
	for (NSView *sub in view.subviews) {
		if (wmViewTreeHasStatusButton(sub)) {
			return YES;
		}
	}
	return NO;
}

// wmIsOurStatusWindow reports whether a left mouse-down landed on this app's
// menu-bar icon. A local monitor only sees events dispatched to THIS process,
// and this app creates exactly one status item, so any status-bar window here
// is ours. Both tests are kept because the private window class name and the
// view hierarchy have each changed across macOS releases.
static BOOL wmIsOurStatusWindow(NSWindow *window) {
	if (window == nil) {
		return NO;
	}
	if ([NSStringFromClass([window class]) containsString:@"StatusBar"]) {
		return YES;
	}
	return wmViewTreeHasStatusButton(window.contentView);
}

// wmInstallTrayMonitor installs the fallback local monitor. Passive: it always
// returns the event unchanged, so Wails' monitor, the button highlight and the
// right-click menu are untouched. Returns 1 once installed.
//
// If the button rewire above succeeded this is redundant, and wmTrayButtonClicked
// is idempotent-safe against both firing (Go debounces on the popover's own
// visibility state).
int wmInstallTrayMonitor(void) {
	if (wmTrayMonitor != nil) {
		return 1;
	}

	// Ups as well as downs, for the reason in wmRewireStatusButtons: the left
	// mouse-DOWN never arrives. Acting on left mouse-UP is what makes this path
	// work; the downs stay in the mask so WM_TRAYDEBUG keeps showing the full
	// picture.
	wmTrayMonitor = [[NSEvent addLocalMonitorForEventsMatchingMask:
		(NSEventMaskLeftMouseDown | NSEventMaskRightMouseDown |
		 NSEventMaskLeftMouseUp | NSEventMaskRightMouseUp)
		handler:^NSEvent *(NSEvent *event) {
			NSWindow *window = event.window;
			BOOL ours = wmIsOurStatusWindow(window);
			if (wmTrayDebug) {
				NSLog(@"wm-traydebug: mouseDown type=%ld ours=%d window=%@ contentView=%@",
					(long)event.type, (int)ours,
					window == nil ? @"(nil)" : NSStringFromClass([window class]),
					window.contentView == nil ? @"(nil)"
						: NSStringFromClass([window.contentView class]));
			}
			// Left mouse-UP is the one that actually gets here. The down case is
			// kept because it costs nothing and Go debounces the pair.
			if (ours && (event.type == NSEventTypeLeftMouseUp ||
			             event.type == NSEventTypeLeftMouseDown)) {
				wmTrayButtonClicked();
			}
			return event;
		}] retain];

	return wmTrayMonitor != nil ? 1 : 0;
}

// wmTestClickStatusButton synthesises a click on the rewired status button.
//
// Diagnostic seam: it isolates "is our target/action wiring correct?" from "does
// the OS deliver clicks to us at all?". If this fires the Go handler but a real
// click does not, the wiring is sound and the events are being swallowed
// upstream. Returns 1 if a button was available to click.
int wmTestClickStatusButton(void) {
	if (wmLastButton == nil) {
		return 0;
	}
	NSLog(@"wm-traydebug: synthesising performClick: on %@ target=%@ action=%@",
		NSStringFromClass([wmLastButton class]),
		wmLastButton.target == nil ? @"(nil)" : NSStringFromClass([wmLastButton.target class]),
		wmLastButton.action == NULL ? @"(nil)" : NSStringFromSelector(wmLastButton.action));
	[wmLastButton performClick:nil];
	return 1;
}

// wmStatusButtonFrameCG returns the rewired status button's frame in global
// CoreGraphics coordinates (origin top-left of the primary display), ready to
// feed to CGEventCreateMouseEvent. Diagnostic seam for the click self-test.
int wmStatusButtonFrameCG(double *x, double *y, double *w, double *h) {
	if (wmLastButton == nil || wmLastButton.window == nil) {
		return 0;
	}
	NSRect f = [wmLastButton.window convertRectToScreen:wmLastButton.frame];
	CGFloat screenH = NSScreen.screens.firstObject.frame.size.height;
	*x = f.origin.x;
	*w = f.size.width;
	*h = f.size.height;
	// Cocoa is bottom-left origin; CG is top-left.
	*y = screenH - (f.origin.y + f.size.height);
	return 1;
}
