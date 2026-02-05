# P6800 - UI: CSS Mask-Based SVG Icons

**Created**: 2025-12-18  
**Updated**: 2025-12-18  
**Priority**: P6800 (Low)  
**Status**: Backlog  
**Depends on**: P4350 (SVG icon system)

---

## Source

- Post: `https://x.com/fractaledmind/status/2001372075559391595`
- Excerpt:
  - “SVG icons have been ‘solved’ myriad ways… Inline SVGs? Bloated DOM. `<img>` tags? Can’t change colors… CSS background-image? Still can’t change colors. But… there’s actually a perfect solution…”

---

## Overview

NixFleet’s current dashboard icon system uses inline SVG symbols plus `<use href="#icon-…">` to get crisp icons that inherit `currentColor`.

This item explores an alternative rendering approach aligned with the post’s stated goals (no DOM bloat + recolorable icons). A strong candidate is **CSS `mask-image` (and `-webkit-mask-image`) backed by external SVG files**, enabling:

- Smaller HTML payload (no inline icon sprite in the DOM)
- Better caching (icons served as static assets)
- Easy recoloring via `currentColor` (by setting `background-color: currentColor`)

### Open Questions

- What exact “perfect solution” does the author recommend (thread follow-ups / linked demo)?
- Are there edge cases we care about (multi-color icons, gradients, hover effects, disabled states)?
- Does Safari behavior match expectations for our target environments?

---

## Problem

Even with a symbol sprite, SVG icons can still have downsides:

- **HTML size / DOM weight**: Inline defs increase response size and DOM complexity.
- **Caching limitations**: Inline defs can’t be cached independently of the HTML.
- **Multi-surface reuse**: If future pages/templates are added, each page must carry the same defs.

---

## Proposal

Introduce a new icon style that renders icons as a masked element:

- Icon element is a `<span class="icon icon--<name>">` (or similar)
- CSS applies:
  - `mask-image: url("/static/icons/<name>.svg")`
  - `-webkit-mask-image: url("/static/icons/<name>.svg")`
  - `mask-repeat: no-repeat; mask-position: center; mask-size: contain;`
  - `background-color: currentColor;`

Keep the existing inline-`<svg><use …></use></svg>` approach as:

- A fallback for environments where mask support is problematic
- A reference implementation while migrating incrementally

---

## Design Notes

- **Single-color only**: `mask-image` is best for monochrome glyphs; multi-color icons would require either inline SVG or multiple layers.
- **Sizing**: ensure all icon sizes can be controlled via CSS (`width`, `height`) similarly to the existing `.icon` rules.
- **Performance**: verify no jank from many separate icon requests; consider bundling (sprite sheet) or HTTP/2 benefits.
- **Accessibility**:
  - Decorative icons should be `aria-hidden="true"`.
  - Icons that convey meaning without text must include accessible labels.

---

## Acceptance Criteria

- [ ] Add a mask-based icon mechanism that supports `currentColor`.
- [ ] Works in modern Chrome/Firefox/Safari (including `-webkit-mask-*`).
- [ ] Dashboard still renders all existing icons with correct size/alignment.
- [ ] Either:
  - [ ] Inline SVG defs are removed from the main template, **or**
  - [ ] A clear migration plan exists (which icons/pages use mask vs inline).
- [ ] No emojis or Unicode-symbol icons.

---

## Related

- **P4350**: UI SVG icon system (current state)
- **P4370**: UI table columns (icon-heavy table view)
