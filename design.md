# Design — qdjr admin

Locked design system for `apps/web`. Every admin page reads this file before
changing UI. Amend this file to grow the system; never override per page.

## Genre
modern-minimal — instrument panel for one person who writes and publishes.

## Macrostructure family
- App pages: **Workbench**. Grouped left rail (240px, md+) · top bar (56px:
  ⌘K pill left, "New post" + user menu right) · content column `max-w-[1200px]`.
  List pages = PageHeader + one table. Editor pages = writing column + 300px
  sticky meta rail.
- Auth pages: centred single card on paper, wordmark above.
- No marketing or content pages live in this app.

## Theme — Cobalt
| Token (shadcn var) | Light | Dark |
|---|---|---|
| paper `--background` | oklch(98.5% 0.004 250) | oklch(18.5% 0.014 260) |
| surface `--card` / `--popover` | oklch(99.3% 0.003 250) | oklch(21.5% 0.016 260) |
| paper-2 `--muted` `--secondary` `--sidebar` | oklch(96.4% 0.006 252) | oklch(24.5% 0.016 260) |
| ink `--foreground` | oklch(24% 0.02 258) | oklch(94% 0.008 255) |
| ink-2 `--muted-foreground` | oklch(48% 0.018 257) | oklch(71% 0.016 256) |
| rule `--border` / `--input` | oklch(90.5% 0.008 255) | oklch(30% 0.016 260) |
| signal `--primary` / `--ring` | oklch(54% 0.20 258) | oklch(72% 0.15 256) |
| signal-ink `--primary-foreground` | oklch(98.5% 0.004 250) | oklch(18.5% 0.014 260) |
| wash `--accent` | oklch(95.5% 0.025 256) | oklch(28% 0.05 258) |
| wash-ink `--accent-foreground` | oklch(38% 0.15 258) | oklch(88% 0.06 256) |
| danger `--destructive` | oklch(55% 0.20 27) | oklch(68% 0.19 24) |
| graphite `--graphite` (code surfaces) | oklch(22% 0.016 260) | oklch(14.5% 0.012 260) |
| graphite-ink `--graphite-foreground` | oklch(92% 0.01 255) | oklch(92% 0.01 255) |

Signal covers under 5% of any screen: primary button, active rail tick,
focus ring, the published dot, link hover underline. Nothing else is blue.
No pure #fff or #000. No gradients, no glass, no shadows (hairlines carry depth).

## Typography
- Display: Space Grotesk 600, tracking -0.02em. Page titles `text-2xl`, desk greeting `text-3xl`. Always roman.
- Body: Inter 400/500, 14px base.
- Mono: JetBrains Mono 400/500 for slugs, dates, IDs, counts, kbd hints, and UPPERCASE labels (`text-[11px] tracking-[0.06em]`).

## Spacing
Tailwind 4pt scale only (`gap-2/3/4/6/8`, `p-4/6`). No arbitrary px except the
documented 240px rail, 300px meta rail, 1200px column.

## Radii
`--radius: 0.375rem` (6px) for buttons, inputs, menus. Code surfaces 10px (`rounded-[10px]`).

## Motion
- Easing `--ease-out: cubic-bezier(0.16, 1, 0.3, 1)`, `--dur-short: 160ms`.
- Allowed: dialog/palette fade, colour transitions on hover. No reveals, no bounce, no parallax.
- `prefers-reduced-motion: reduce` → no transforms (shadcn `animate-in` zoom disabled).

## Microinteractions stance
Silent success: toasts only for errors and for actions with no visible result.
Focus rings appear instantly, 2px signal at ≥3:1. Keyboard: ⌘K / Ctrl+K opens
the palette everywhere in `/admin`; ↑/↓ move, Enter runs, Esc closes.

## Copy
Sentence case. Buttons name the action ("Save", "Publish", "New post").
Counts and dates are real API values or `—`; never invented.

## Exports
Tokens live in `apps/web/app/globals.css` (`:root`, `.dark`, `@theme inline`).
