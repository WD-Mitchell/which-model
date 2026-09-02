//go:build darwin && !ios

// Manages macOS Dock icon visibility and activation policy transitions.
//
// which-model is primarily a menu-bar app running under
// NSApplicationActivationPolicyAccessory. When the Settings window is opened,
// switching to NSApplicationActivationPolicyRegular adds the app to the Dock and
// Cmd-Tab task switcher so users can switch back to Settings when it is obscured.
// When Settings closes, switching back to NSApplicationActivationPolicyAccessory
// removes the Dock icon.

#import <Cocoa/Cocoa.h>

void wmSetDockIconVisible(int visible) {
	void (^block)(void) = ^{
		if (NSApp == nil) {
			return;
		}
		NSApplicationActivationPolicy targetPolicy = visible
			? NSApplicationActivationPolicyRegular
			: NSApplicationActivationPolicyAccessory;

		if ([NSApp activationPolicy] != targetPolicy) {
			[NSApp setActivationPolicy:targetPolicy];
			if (visible) {
				[NSApp activateIgnoringOtherApps:YES];
			}
		}
	};

	if ([NSThread isMainThread]) {
		block();
	} else {
		dispatch_async(dispatch_get_main_queue(), block);
	}
}

int wmGetDockIconVisible(void) {
	if (NSApp == nil) {
		return 0;
	}
	return [NSApp activationPolicy] == NSApplicationActivationPolicyRegular ? 1 : 0;
}
