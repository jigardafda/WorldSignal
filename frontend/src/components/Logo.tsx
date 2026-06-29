/** The WorldSignal brand mark: a gradient badge with a globe + signal arcs.
 * Mirrors assets/icon.svg (and the favicon) so the in-app logo matches the
 * project branding. `size` is the rendered square size in pixels. */
export function LogoMark({ size = 28, title = "WorldSignal" }: { size?: number; title?: string }) {
  return (
    <svg width={size} height={size} viewBox="0 0 128 128" fill="none" xmlns="http://www.w3.org/2000/svg" role="img" aria-label={title}>
      <defs>
        <linearGradient id="ws-logo-grad" x1="16" y1="12" x2="112" y2="116" gradientUnits="userSpaceOnUse">
          <stop stopColor="#2F6DF6" />
          <stop offset="1" stopColor="#22C3E6" />
        </linearGradient>
      </defs>
      <rect x="8" y="8" width="112" height="112" rx="28" fill="url(#ws-logo-grad)" />
      <g stroke="#FFFFFF" strokeWidth="5" strokeLinecap="round" fill="none" opacity="0.96">
        <circle cx="52" cy="76" r="30" />
        <ellipse cx="52" cy="76" rx="13" ry="30" />
        <line x1="22" y1="76" x2="82" y2="76" />
      </g>
      <circle cx="88" cy="40" r="7" fill="#FFFFFF" />
      <g stroke="#FFFFFF" strokeWidth="5" strokeLinecap="round" fill="none">
        <path d="M98 30 a16 16 0 0 1 0 20" opacity="0.85" />
        <path d="M105 23 a26 26 0 0 1 0 34" opacity="0.55" />
      </g>
    </svg>
  );
}
