//go:build darwin && !ios

// The menu-bar item: the recommended provider's mark, then two lines of text —
// the profile on top, the model it picked below — composed BY HAND into a
// single template image.
//
// WHY AN IMAGE AND NOT A TITLE. An NSStatusItem's title is an NSButton title,
// and a button draws one line: a "\n" renders as a space. Two lines can be had
// from an attributed title with a forced line height, and that is where this
// started — but NSButtonCell then owns the vertical placement, and it puts the
// pair high: measured on a 22pt bar, 0.3pt of air above the caps and 1.5pt
// below the descenders. There is no attribute that moves it (baseline offset is
// ignored once min/max line height are pinned), so the only way to centre the
// text — and to spend the whole bar on type rather than on the cell's idea of
// leading — is to draw it.
//
// Drawing it also removes the second writer: Wails' SetLabel assigns
// button.title from a dispatch_async block, and its SetTemplateIcon assigns
// button.image, either of which would land on top of ours. tray.go therefore
// calls NEITHER on macOS; this file sets button.image and nothing else does.
//
// The image is a TEMPLATE image, so only its alpha matters: AppKit tints the
// whole composition — mark and text alike — in the menu bar's own foreground
// colour, which is what makes it track light/dark, the highlight state while
// the menu is open, and Reduce Transparency, without this code observing any
// of them.
//
// The status item is found the same way the click repair finds it
// (traybutton_darwin.m): NSStatusBar's private _items, guarded, because the
// status window is system-hosted and never appears in NSApp.windows.

#import <Cocoa/Cocoa.h>
#include <stdlib.h>

// Type metrics, in points, for the two-line stack.
//
// The menu bar is 22pt tall ([NSStatusBar thickness]) and that is the whole
// budget. The visible block runs from the first line's cap height down to the
// second line's descender, so its height is capHeight + baselineGap +
// |descender| — at 11pt that is 21.1pt, leaving half a point of air above and
// below, and 0.9pt between the first line's descenders and the second line's
// caps. 11.5pt closes that gap to 0.5pt and the lines start to touch, so this
// is the practical ceiling for two lines.
static const CGFloat wmTrayTitleFontSize = 11.0;
static const CGFloat wmTrayTitleBaselineGap = 11.0;

// Horizontal metrics: the mark gets a square slot the height of the bar (its
// own art is already inset — see assets/providers/PROVENANCE.md), then a hair
// of air before the text. wmTrayTitleMaxWidth stops a pathological model name
// from eating the menu bar; past it the line truncates with an ellipsis.
static const CGFloat wmTrayTitleGap = 2.0;
static const CGFloat wmTrayTitleMaxWidth = 260.0;

// wmTrayTitleAttributes are the run attributes for one line. Both lines share
// them: at menu-bar scale a lighter or smaller second line read as washed out
// rather than as secondary. The colour is irrelevant beyond its alpha (see the
// template note above); black is the conventional choice for template art.
static NSDictionary *wmTrayTitleAttributes(void) {
	NSMutableParagraphStyle *style = [[[NSMutableParagraphStyle alloc] init] autorelease];
	style.alignment = NSTextAlignmentLeft;
	style.lineBreakMode = NSLineBreakByTruncatingTail;
	return @{
		NSFontAttributeName: [NSFont systemFontOfSize:wmTrayTitleFontSize
		                                       weight:NSFontWeightRegular],
		NSForegroundColorAttributeName: [NSColor blackColor],
		NSParagraphStyleAttributeName: style,
	};
}

// wmTrayBarThickness is the menu bar's height — the canvas every measurement
// below is centred in.
static CGFloat wmTrayBarThickness(void) {
	CGFloat thickness = [[NSStatusBar systemStatusBar] thickness];
	return thickness > 0 ? thickness : 22.0;
}

// wmStatusItemImage composes the status item's picture: `mark` (may be nil) in
// a square slot on the left, then `top` over `bottom`. Returns nil when there
// is nothing at all to draw.
//
// VERTICAL PLACEMENT. Each line is drawn on its own, at a computed baseline,
// rather than as one string with a forced line height — that is what puts the
// ink where we say. Calibrated against AppKit's own layout: a single line drawn
// with its fragment origin at y sits with its baseline at
// y + (naturalLineHeight - ascender), measured exactly on macOS 27 and matching
// the font metrics. From there:
//
//	inkHeight = capHeight + baselineGap + |descender|
//	baseline1 = (barHeight + inkHeight)/2 - capHeight   // centres the ink
//	baseline2 = baseline1 - baselineGap
//
// so the air above the caps and below the descenders is equal by construction,
// at any font size and on any bar thickness.
NSImage *wmStatusItemImage(NSString *top, NSString *bottom, NSImage *mark) {
	NSDictionary *attrs = wmTrayTitleAttributes();
	NSFont *font = attrs[NSFontAttributeName];
	CGFloat barHeight = wmTrayBarThickness();

	NSAttributedString *line1 = top.length > 0
		? [[[NSAttributedString alloc] initWithString:top attributes:attrs] autorelease]
		: nil;
	NSAttributedString *line2 = bottom.length > 0
		? [[[NSAttributedString alloc] initWithString:bottom attributes:attrs] autorelease]
		: nil;

	CGFloat textWidth = 0;
	if (line1 != nil) textWidth = MAX(textWidth, [line1 size].width);
	if (line2 != nil) textWidth = MAX(textWidth, [line2 size].width);
	textWidth = ceil(MIN(textWidth, wmTrayTitleMaxWidth));

	CGFloat iconWidth = mark != nil ? barHeight : 0;
	CGFloat textX = iconWidth + (iconWidth > 0 && textWidth > 0 ? wmTrayTitleGap : 0);
	CGFloat width = textX + textWidth;
	if (width <= 0) {
		return nil;
	}

	// Natural line box height for one line, which is what drawInRect: lays out
	// in when the paragraph style leaves the line height alone.
	CGFloat lineBox = line1 != nil ? [line1 size].height : (line2 != nil ? [line2 size].height : 0);
	CGFloat inkHeight = font.capHeight + wmTrayTitleBaselineGap + fabs(font.descender);
	if (line1 == nil || line2 == nil) {
		// One line: its own ink, cap height to descender.
		inkHeight = font.capHeight + fabs(font.descender);
	}
	CGFloat baseline1 = (barHeight + inkHeight) / 2.0 - font.capHeight;
	CGFloat originGap = lineBox - font.ascender; // fragment origin -> baseline
	CGFloat origin1 = baseline1 - originGap;
	CGFloat origin2 = origin1 - wmTrayTitleBaselineGap;

	NSImage *image = [NSImage imageWithSize:NSMakeSize(width, barHeight)
	                                flipped:NO
	                         drawingHandler:^BOOL(NSRect dst) {
		if (mark != nil) {
			[mark drawInRect:NSMakeRect(0, 0, iconWidth, barHeight)
			        fromRect:NSZeroRect
			       operation:NSCompositingOperationSourceOver
			        fraction:1.0];
		}
		if (line1 != nil && line2 != nil) {
			[line1 drawInRect:NSMakeRect(textX, origin1, textWidth, lineBox)];
			[line2 drawInRect:NSMakeRect(textX, origin2, textWidth, lineBox)];
		} else if (line1 != nil || line2 != nil) {
			[(line1 != nil ? line1 : line2)
				drawInRect:NSMakeRect(textX, origin1, textWidth, lineBox)];
		}
		return YES;
	}];
	[image setTemplate:YES];
	return image;
}

// wmSetStatusTitleTwoLine draws `top` over `bottom`, behind the provider mark in
// `icon` (SVG bytes; may be NULL), on every status button this app owns — there
// is exactly one. Empty strings and a NULL icon clear the item.
//
// Returns the number of buttons updated. 0 means the status item does not exist
// yet and the caller should retry (traytitle_darwin.go).
int wmSetStatusTitleTwoLine(const char *top, const char *bottom, const void *icon, int iconLen) {
	@autoreleasepool {
		NSString *topLine = top == NULL ? @"" : [NSString stringWithUTF8String:top];
		NSString *bottomLine = bottom == NULL ? @"" : [NSString stringWithUTF8String:bottom];

		NSImage *mark = nil;
		if (icon != NULL && iconLen > 0) {
			NSData *data = [NSData dataWithBytes:icon length:(NSUInteger)iconLen];
			// AppKit decodes SVG into an NSImage directly (_NSSVGImageRep,
			// macOS 13+). A mark that fails to decode is simply absent.
			mark = [[[NSImage alloc] initWithData:data] autorelease];
			if (mark != nil && !mark.isValid) {
				mark = nil;
			}
		}

		NSArray *items = nil;
		@try {
			// Private, read-only and guarded, as in traybutton_darwin.m: if the
			// key ever disappears this raises, we swallow it, and the menu bar
			// keeps whatever it is already showing.
			items = [[NSStatusBar systemStatusBar] valueForKey:@"_items"];
		} @catch (NSException *e) {
			NSLog(@"wm-tray: NSStatusBar _items unavailable: %@", e.reason);
			return 0;
		}

		NSImage *composed = wmStatusItemImage(topLine, bottomLine, mark);

		int updated = 0;
		for (id item in items) {
			if (![item isKindOfClass:[NSStatusItem class]]) {
				continue;
			}
			NSStatusBarButton *button = [(NSStatusItem *)item button];
			if (button == nil) {
				continue;
			}
			// The composition IS the button's content: no title is set, so
			// nothing of AppKit's own text layout survives to fight it.
			button.title = @"";
			button.attributedTitle = [[[NSAttributedString alloc] initWithString:@""] autorelease];
			button.image = composed;
			button.imagePosition = composed != nil ? NSImageOnly : NSNoImage;
			updated++;

			// WM_TRAYDEBUG=1 (the switch traybutton_darwin.m reads) reports the
			// composed size, which is the proof the stack fits the bar.
			if (getenv("WM_TRAYDEBUG") != NULL) {
				NSLog(@"wm-traydebug: status image %@ for %@ / %@",
					composed == nil ? @"(nil)" : NSStringFromSize(composed.size),
					topLine, bottomLine);
			}
		}
		return updated;
	}
}
