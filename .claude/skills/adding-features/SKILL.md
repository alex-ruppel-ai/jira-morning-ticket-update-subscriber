---
name: adding-features
description: "You MUST use this before any creative work - creating features, building components, adding functionality, or modifying behavior. Explores user intent, requirements and design before implementation."
---

# New Feature Requests

When a user asks for a new feature, implement it as a **new tab** in the frontend UI. The pattern is:

1. Create a new component in `frontend/src/components/` (e.g., `MyFeature.tsx`)
2. Add the tab to `frontend/src/components/Sidebar.tsx` — follow the existing tab pattern (icon + label + tab ID). **Place the new feature's tab first in the tab list.**
3. Render the component in `frontend/src/App.tsx` — add a case to the `activeTab` conditional. Set the new feature's tab as the default `activeTab`.
4. If the feature requires backend data, add an API endpoint in `api.go` and a corresponding Slack handler in `main.go`
5. **Remove any existing components, tabs, and backend handlers that are not needed for the requested feature.** For example, if the app does not use Slack messaging, remove `SendMessage.tsx`, `SendDM.tsx`, and their corresponding routes/handlers. If it does not use Anaheim, remove `Anaheim.tsx` and related code. Keep only what is necessary for the feature being built.

**When removing code, follow these rules to avoid compile errors:**
- Remove the full initialization block for any package you're removing — including variable declarations and `err` assignments. Go will not compile if a variable is declared but never used.
- After removing a package from one file, search **all** `.go` files for references to that package and remove them too. A common mistake is removing an import from `main.go` but leaving references to it in `api.go`.
- After making removals, mentally trace each import in every `.go` file and confirm it is still referenced in that file. Remove any import that is no longer used