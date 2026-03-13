# Menu UI Variations

Three distinct navigation styles for the main app menu. Users can swap between them via theme swap buttons in the header or settings.

---

## Variation 1: **Minimal Pill**

**Concept:** Clean, understated pill buttons in a soft container. Current aesthetic refined.

**Visual:**
- Nav items in a single rounded pill-shaped container (bg-tertiary)
- Each button: subtle rounded corners, transparent → light fill on hover/active
- No borders, minimal shadow
- Typography: medium weight, secondary color → primary on active
- Compact padding, tight gaps

**Feel:** Calm, professional, unobtrusive. Good for focus.

**CSS approach:** `[data-menu-style="minimal"]` — refine existing `.app-nav` pill treatment.

---

## Variation 2: **Bold Tabs**

**Concept:** Strong, tab-bar style with clear active state. Browser-tab energy.

**Visual:**
- Horizontal row of tabs with bottom border on container
- Active tab: bold underline (2–3px) in accent color, or filled background
- Inactive: muted text, no underline
- Slight elevation or border on nav container
- More padding, clearer tap targets
- Optional: subtle top border radius on tabs (attached to content below)

**Feel:** Confident, decisive. Clear “you are here” feedback.

**CSS approach:** `[data-menu-style="bold"]` — tab bar, underline indicator, stronger contrast.

---

## Variation 3: **Stacked Cards**

**Concept:** Each nav item as a small card. More visual separation, slightly playful.

**Visual:**
- Nav items as individual cards with border, shadow, rounded corners
- Grid or flex wrap; cards can stack on narrow screens
- Active: accent border or filled background, slightly elevated
- Hover: lift + shadow
- Icons optional (e.g. dumbbell, list, chart) for future enhancement
- More whitespace between items

**Feel:** Modular, tactile. Each destination feels like a distinct choice.

**CSS approach:** `[data-menu-style="stacked"]` — card-style `.nav-button`, gap, shadow.

---

## Theme Swap Controls

- **Location:** Header (next to theme toggle), inside Settings, and **login screen** (top-right corner)
- **UI:** 3 small buttons or icons representing each style:
  - `○` Minimal
  - `▬` Bold  
  - `▢` Stacked
- **Persistence:** `localStorage` key `liftoff-menu-style`
- **Default:** `minimal`

---

## Login Screen

The same 3 variations apply to the auth pages (Login, Register, Forgot Password, Reset Password):
- **Minimal:** Soft card, subtle inputs and links
- **Bold:** Stronger borders, underlined links on hover
- **Stacked:** Card-style buttons and links with shadow

Swap buttons (○ ▬ ▢) appear in the top-right corner of the login screen.

## Implementation Notes

- All variations use existing semantic structure (`.app-nav`, `.nav-button`; `.auth-card`, `.auth-button`, `.auth-link`)
- `data-menu-style` on `body` or `.app` drives variation
- Responsive: stacked/stacked-cards adapt to column layout on mobile
- Disabled state (e.g. Active Session) styled consistently across variations
