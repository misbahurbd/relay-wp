# Design system and UX rules

The web app's look comes from the ChatPulse design; its rules come from WCAG 2.2 AA and common UX research. Tokens live in [`apps/web/src/styles/app.css`](../apps/web/src/styles/app.css), primitives in `apps/web/src/components/ui/` (shadcn, restyled). Every new screen follows the rules below; a change that breaks one needs a reason in the PR.

---

## 1. Tokens

| Group    | Tokens (Tailwind: `bg-*`, `text-*`, `border-*`)                                         | Use for                                                        |
| -------- | --------------------------------------------------------------------------------------- | -------------------------------------------------------------- |
| Surfaces | `background`, `card`, `popover`, `muted`, `accent`, `secondary`                         | Page, cards and menus, quiet fills, hover                      |
| Text     | `foreground`, `muted-foreground`                                                        | Body text, secondary text and placeholders                     |
| Lines    | `border` (decorative), `input` (form controls)                                          | Cards and dividers / input, select, checkbox, radio outlines   |
| Action   | `primary`, `primary-foreground`, `ring`                                                 | Main buttons, checked controls, focus outline                  |
| Brand    | `brand`, `brand-soft`, `brand-text`                                                     | Progress, selected chips, success                              |
| Status   | `info`, `warning`, `destructive`, `neutral`, `purple`, each with `-soft`; `warning-bar` | Badges, alerts, banners                                        |
| Product  | `meta`, `panel`, `chat`, `bubble-in`, `bubble-out`, `tick-sent`, `tick-read`, `overlay` | Facebook/Meta buttons, auth marketing panel, WhatsApp previews |
- Colours only through tokens; no raw hex in components.
- Radius: `rounded-sm` 8px, `rounded-md` 10px, `rounded-lg` 12px (controls), `rounded-xl` 16px (cards). Shadow: `shadow-card`.
- Type: Plus Jakarta Sans, 14px body, line height 1.45. Headings extra-bold with tight tracking. Page title 28px, section title 19–20px, card title 16px.
- Light and dark are both required. Check every screen in both.

## 2. Accessibility (WCAG 2.2 AA)

| Rule                              | How the system meets it                                                                                                                                                     |
| --------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Text contrast ≥ 4.5:1 (1.4.3)     | Every text/background token pair is ≥ 5:1 in both themes. Don't put `muted-foreground` on `muted` for important text.                                                       |
| Control outlines ≥ 3:1 (1.4.11)   | `input` is 3.4:1 against `card`. Use `border-input` on anything a user types into or ticks; `border` is for decoration only.                                                |
| Visible focus (2.4.7, 2.4.13)     | Primitives draw a solid 2px `ring` outline, offset 2px, on `:focus-visible`; a global fallback covers links and custom widgets. Never `outline-none` without a replacement. |
| Focus not hidden (2.4.11)         | Sticky headers and banners must not cover the focused element; add `scroll-padding-top` when a sticky header is added.                                                      |
| Target size ≥ 24×24px (2.5.8)     | Buttons, inputs and chips are 36–50px. Checkbox and radio have a 26px hit area; wrap them in their `<Label>` so the text is clickable too.                                  |
| Don't rely on colour (1.4.1)      | Status always has text or an icon as well as colour (badges, alerts, errors).                                                                                               |
| Reduced motion (2.3.3)            | `prefers-reduced-motion` turns animations and transitions off globally.                                                                                                     |
| Accessible authentication (3.3.8) | Allow paste and password managers everywhere. Code inputs accept a pasted SMS or email and keep only the digits.                                                            |
| Redundant entry (3.3.7)           | Never ask twice for what the user already gave us (e.g. name from Google, business name into the WhatsApp display name).                                                    |
| Consistent help (3.2.6)           | Help links ("Chat with support", "Book a setup call") stay in the same place on every setup screen.                                                                         |

## 3. Forms

- Every field has a visible `<Label>`; placeholders are examples, never the label.
- One column. Group related fields; mark optional fields "(optional)" rather than marking required ones.
- Right input types and autofill: `type="email"` + `autocomplete="email"`, `autocomplete="new-password"` / `"current-password"`, `autocomplete="name"`, `type="tel"` + `autocomplete="tel"`, `autocomplete="one-time-code"` (the OTP primitive sets it).
- Validate a field when the user leaves it, not on every keystroke; re-validate on change once it has shown an error. Validate everything again on submit and move focus to the first invalid field.
- Errors go **below** the field (`FieldError`), in `text-destructive`, linked with `aria-describedby`, and the field gets `aria-invalid`. Say what to do, without blame: "Use at least 8 characters", not "Invalid password".
- Show password rules before the user types, and a show/hide toggle on password fields.
- Prefer an enabled button that explains what's missing over a disabled one with no reason. If a button must be disabled (e.g. "Continue" until Meta finishes), say why next to it.
- Keep entered data on every error, Back and refresh (the onboarding API saves each step).

## 4. Feedback and states

- Every async action shows progress within 100 ms (spinner in the button, button disabled to prevent double submit) and a clear result.
- Empty states say what goes there and offer the one action that fills it ("No contacts yet" → "Import contacts").
- Loading: skeletons that match the final layout for content; spinners only inside buttons.
- Toasts (`sonner`) only for confirmations that need no action ("Link copied"). Errors that need action stay on the page.
- Banners for account-wide states (trial, WhatsApp disconnected), one at a time, most urgent first.

## 5. Onboarding

- Shortest path to the first value: account → connect WhatsApp → first message. Anything else (team, templates, contacts) waits for the checklist.
- Show progress and time: "Step 2 of 3 · about 3 minutes left".
- Ask only what changes the experience, and say why ("We use this to suggest templates").
- Every step can be left and resumed with answers saved; skippable steps say what the user gives up.
- Explain before asking for something hard (Meta login, a phone number): what they need, how long it takes, what happens.
- Tours are short (≤ 4 steps), skippable, and replayable.

## 6. Layout and responsiveness

- Mobile first; every screen works at 360px wide with a 16px gutter and no horizontal scroll.
- Inputs are 16px text on small screens (`text-base md:text-sm`), so iOS doesn't zoom in.
- Comfortable line length for prose: `max-w-prose` (≈ 65 characters).
- Spacing comes from the 4px scale (`gap-2`, `gap-4`, `p-6`); no one-off pixel values.

## Sources

- [What's new in WCAG 2.2](https://www.w3.org/WAI/standards-guidelines/wcag/new-in-22/) (W3C)
- [Deque: WCAG 2.2 updates](https://dequeuniversity.com/resources/wcag-2.2)
- [Accessible authentication](https://frontendchecklist.io/rules/accessibility/accessible-authentication) and [simple one-time passcode inputs](https://cloudfour.com/thinks/simple-one-time-passcode-inputs/)
- [web.dev: SMS OTP form best practices](https://web.dev/articles/sms-otp-form)
- [Yukon: helping people recover from a form error](https://guide.yukon.ca/en/content-and-design/forms/user-experience-ux-standards-forms/how-help-people-recover-form-error)
- [UXPin: error feedback on mobile forms](https://www.uxpin.com/studio/blog/error-feedback-best-practices-mobile-forms/)
- [SaaS onboarding flows that convert (2026)](https://saasui.design/blog/saas-onboarding-flows-that-actually-convert-2026)
