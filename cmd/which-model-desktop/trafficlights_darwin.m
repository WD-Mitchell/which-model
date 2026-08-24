//go:build darwin && !ios

// Positions the settings window's traffic lights so their padding is even on
// the top, bottom and left.
//
// AppKit pins the standard window buttons to ITS titlebar, not ours. With
// MacTitleBarHidden the native titlebar is the system's 28pt one, so the 12pt
// buttons land ~8pt from the window top and ~20pt from its left edge — while
// the page draws a 40px title row underneath. The result reads as too much
// space below the buttons and too much to their left: three different gaps.
//
// Neither Wails nor AppKit exposes a knob for this (Electron's
// trafficLightPosition has no counterpart here), so the buttons are moved
// directly. They are ordinary NSButtons in a shared superview, so setting each
// frame origin is enough — and AppKit re-lays them out on resize and on
// enter/exit fullscreen, which is why the Go side re-applies this on those
// events rather than once at creation.

#import <Cocoa/Cocoa.h>

// wmPositionTrafficLights insets the button cluster by `inset` from the window's
// left and top edges, preserving AppKit's own spacing between the three.
//
// Even padding on all three sides therefore means the page's title row must be
// (inset + buttonHeight + inset) tall — SettingsShell.module.css pins it to 40px
// against an inset of 14 and AppKit's 12pt buttons.
//
// Returns 1 when the buttons were found and moved. Must run on the main thread.
int wmPositionTrafficLights(void *nsWindow, double inset) {
	NSWindow *window = (NSWindow *)nsWindow;
	if (window == nil) {
		return 0;
	}

	NSButton *close = [window standardWindowButton:NSWindowCloseButton];
	NSButton *miniaturize = [window standardWindowButton:NSWindowMiniaturizeButton];
	NSButton *zoom = [window standardWindowButton:NSWindowZoomButton];
	if (close == nil || close.superview == nil) {
		return 0;
	}

	NSView *container = close.superview;
	CGFloat containerHeight = container.frame.size.height;
	CGFloat buttonHeight = close.frame.size.height;

	// The container's origin is bottom-left and its top edge is the window's, so
	// a gap of `inset` BELOW the window top is this y in container space.
	CGFloat y = containerHeight - inset - buttonHeight;

	// Preserve the system's horizontal rhythm rather than inventing one: the
	// existing centre-to-centre delta is the spacing macOS users expect.
	CGFloat spacing = 20.0;
	if (miniaturize != nil) {
		CGFloat delta = miniaturize.frame.origin.x - close.frame.origin.x;
		if (delta > 0) {
			spacing = delta;
		}
	}

	NSButton *buttons[3] = {close, miniaturize, zoom};
	for (int i = 0; i < 3; i++) {
		NSButton *button = buttons[i];
		if (button == nil) {
			continue;
		}
		NSRect frame = button.frame;
		frame.origin.x = inset + (spacing * i);
		frame.origin.y = y;
		[button setFrame:frame];
	}
	return 1;
}
