# Technical Debt and Future Work

## Theme System: Option B — Move styles onto model struct

**Status:** Deferred. Current implementation uses Option A (mutable package-level vars via `ApplyTheme()`).

### Background

Themes currently work by having `ApplyTheme()` in `internal/ui/theme.go` mutate
package-level style vars (`titleStyle`, `headerStyle`, etc.) across `palette.go`
and `interactive.go`. This is simple and safe because Bubble Tea is single-threaded,
so there are no data races.

Option B would move all lipgloss style definitions off package-level vars and onto
a `Styles` struct stored directly on the Bubble Tea `model` (`m.styles`).

### Benefits of Option B

- No global mutable state — styles live where they're used
- Enables per-session style isolation (useful if multiple programs share a process)
- Easier to unit test the view layer (pass a `Styles` value, assert output)
- Clearer data flow: `ApplyTheme(t)` → `m.styles = BuildStyles(t)` with no side effects

### What needs to change

1. **Define `type Styles struct`** in `internal/ui/theme.go` with one field per current
   package-level style var (e.g. `Title`, `Header`, `Info`, `Footer`, `App`, `NormalTitle`,
   `SelectedTitle`, `Category`, `Hint`, `Shortcut`, `SelectedShortcut`, etc.).

2. **Add `styles Styles` field** to `model` struct in `internal/ui/interactive.go`.

3. **Replace all package-level style refs in `interactive.go`** — roughly 40 references —
   with `m.styles.X`. For example:
   - `titleStyle.Render(...)` → `m.styles.Title.Render(...)`
   - `headerStyle.Width(...).Render(...)` → `m.styles.Header.Width(...).Render(...)`

4. **Update `itemDelegate.Render()` in `palette.go`** — the delegate currently reads
   package-level vars (`normalTitleStyle`, `selectedTitleStyle`, etc.). These need to be
   threaded in via a closure or a package-level accessor that reads from the model.
   One clean approach: make `itemDelegate` carry a `*Styles` pointer.

5. **Remove the now-unnecessary package-level color and style vars** from `palette.go`
   and `interactive.go`. `ApplyTheme` becomes a pure function:
   ```go
   func BuildStyles(t Theme) Styles { ... }
   ```

6. **Update `LaunchInteractive`** to call `m.styles = BuildStyles(activeTheme)` instead
   of `ApplyTheme(activeTheme)`.

7. **Update `openThemesPalette` live preview** to set `m.styles = BuildStyles(t)` rather
   than calling `ApplyTheme(t)`.

### When to do it

Migrate to Option B when:
- Adding user-customizable theme overrides (per-field color editing)
- Writing unit tests for the TUI view layer
- Running multiple concurrent TUI programs in the same process
