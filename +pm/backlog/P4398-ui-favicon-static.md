# P4398 - Static Assets (Favicon, Logo)

**Priority**: High  
**Status**: Done  
**Effort**: Small  
**References**: `+pm/legacy/v1.0/base.html`

## Problem

v2 is missing static assets:

- No favicon (browser tab shows generic icon)
- No logo image for header
- No watermark background

## Solution

### Files to Copy

From v1 `app/static/`:

- `nixfleet_favicon.png` - Browser favicon (32x32)
- `nixfleet_fade_1k.png` - Logo image (transparent background)

### Serve Static Files

Add static file serving to Go server:

```go
// In server.go
r.Handle("/static/*", http.StripPrefix("/static/",
    http.FileServer(http.Dir("static"))))
```

### Favicon in HTML

```html
<link rel="icon" type="image/png" href="/static/nixfleet_favicon.png" />
```

### Logo in Header

```html
<img src="/static/nixfleet_fade_1k.png" alt="NixFleet" class="brand-logo" />
```

### Background Watermark

```css
body::before {
  content: "";
  position: fixed;
  top: 25%;
  left: 50%;
  transform: translate(-50%, -25%);
  width: 500px;
  height: 440px;
  background: url("/static/nixfleet_fade_1k.png") no-repeat top center;
  background-size: 100% 100%;
  opacity: 0.3;
  pointer-events: none;
  z-index: -1;
}
```

### Directory Structure

```
v2/
├── static/
│   ├── nixfleet_favicon.png
│   └── nixfleet_fade_1k.png
└── cmd/
    └── nixfleet-dashboard/
        └── main.go
```

### Docker Considerations

Copy static files in Dockerfile:

```dockerfile
COPY v2/static /app/static
```

### Requirements

- [x] Copy favicon from v1
- [x] Copy logo from v1
- [x] Add static file serving route
- [x] Add favicon link to base template
- [x] Add logo to header
- [x] Add background watermark
- [x] Update Dockerfile to include static files

## Related

- P4355 (Header) - Uses logo
- P4360 (Footer) - Uses small favicon icon
