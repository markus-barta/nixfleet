# P4360 - UI: Footer

**Priority**: Medium  
**Status**: Pending  
**Effort**: Small  
**References**: `+pm/legacy/v1.0/dashboard.html` (lines 2782-2816)

## Problem

v2 has no footer. v1 had a professional footer with:

- Version number
- GitHub source link
- License info
- Server hostname indicator
- "Made with ❤️" credits

## Solution

### Footer Structure

```html
<footer class="site-footer">
  <div class="footer-left">
    <span>NixFleet {{ version }}</span>
    <a href="https://github.com/markus-barta/nixfleet" class="footer-link">
      <svg><use href="#icon-github" /></svg>
      Source
    </a>
    <a href="https://www.gnu.org/licenses/agpl-3.0.html" class="footer-link">
      <svg><use href="#icon-license" /></svg>
      AGPL-3.0
    </a>
  </div>
  <div class="footer-center">
    <img src="/static/nixfleet_favicon.png" alt="" width="12" height="12" />
    <span>{{ server_hostname }}</span>
  </div>
  <div class="footer-right">
    <span class="made-with">
      Made with <svg class="heart"><use href="#icon-heart" /></svg> by
      <a href="https://x.com/markusbarta">@markusbarta</a>, Claude & Cursor in
      <a
        href="https://maps.google.com/?q=Graz,Austria"
        class="footer-location-link"
      >
        <svg><use href="#icon-home" /></svg>
        Graz
      </a>
    </span>
  </div>
</footer>
```

### CSS

```css
.site-footer {
  margin-top: 2rem;
  padding: 1rem 0;
  border-top: 1px solid var(--border);
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 0.7rem;
  color: var(--fg-gutter);
}

.made-with .heart {
  color: var(--red);
  animation: heartbeat 1.5s ease-in-out infinite;
}

@keyframes heartbeat {
  0%,
  100% {
    transform: scale(1);
  }
  50% {
    transform: scale(1.15);
  }
}
```

### Requirements

- [ ] Add footer to base template
- [ ] Pass version and server_hostname to template
- [ ] Add favicon to static files
- [ ] Style footer with flexbox layout
- [ ] Add animated heart icon
- [ ] Responsive: stack vertically on mobile

## Related

- P4350 (Icons) - Needs heart, github, license, home icons
