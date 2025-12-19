# Understanding Flake Updates in NixFleet

## The Three Compartments Explained

```
┌──────────┬──────────┬──────────┐
│   Git    │   Lock   │  System  │
└──────────┴──────────┴──────────┘
```

### 🔀 Git Compartment

**Question it answers:** "Is my local repo up to date with GitHub?"

- **Green**: Local repo matches `origin/main`
- **Yellow**: Local repo is behind (need to `git pull`)

**How it works:** Dashboard compares the agent's reported generation (commit hash) with the latest commit on GitHub.

---

### 🔒 Lock Compartment

**Question it answers:** "How old is my `flake.lock` file?"

- **Green**: Updated within the last 7 days
- **Yellow (8-30 days)**: Consider updating
- **Yellow (>30 days)**: Needs update

**What is `flake.lock`?**

```
nixcfg/
├── flake.nix          ← Defines WHAT inputs you use (nixpkgs, home-manager, etc.)
└── flake.lock         ← Pins WHICH VERSION of each input (specific git commits)
```

The `flake.lock` is like a `package-lock.json` or `Cargo.lock` — it freezes your dependencies to specific versions.

**Why update it?**

- Get security patches from nixpkgs
- Get new package versions
- Get bug fixes from home-manager, etc.

---

### ❄️ System Compartment

**Question it answers:** "Does my running system match what the config would build?"

- **Green**: Running system = what flake would build
- **Yellow**: Running system is outdated (need to `switch`)

**How it works:** Compares `/run/current-system` with `nix build --dry-run` output.

---

## The Update Workflow (Current - Manual)

```
┌─────────────────────────────────────────────────────────────────┐
│  1. GitHub Action runs weekly                                   │
│     └── Runs `nix flake update` (bumps all inputs)              │
│     └── Creates a PR with the new flake.lock                    │
│                                                                 │
│  2. You manually review and merge the PR on GitHub              │
│                                                                 │
│  3. You click "Pull" in NixFleet dashboard                      │
│     └── Each host does `git pull` to get new flake.lock         │
│                                                                 │
│  4. You click "Switch" in NixFleet dashboard                    │
│     └── Each host rebuilds with new packages                    │
└─────────────────────────────────────────────────────────────────┘
```

### The Pain Points

1. **The Lock compartment only sees the deployed flake.lock**
   - It doesn't know there's a PR waiting on GitHub
   - It just measures "how old is the file I have"

2. **You have to manually merge the PR**
   - Go to GitHub → find the PR → review → merge
   - Then go back to NixFleet → Pull → Switch

3. **No visibility into pending updates**
   - Dashboard doesn't show "hey, there's an update PR waiting"

---

## The Ideal Workflow (P4300 Goal)

```
┌─────────────────────────────────────────────────────────────────┐
│  1. GitHub Action creates update PR (same as before)            │
│                                                                 │
│  2. NixFleet dashboard detects the PR                           │
│     └── Lock compartment shows "Update PR pending"              │
│     └── Badge or notification appears                           │
│                                                                 │
│  3. You click "Merge & Deploy" in NixFleet                      │
│     └── Dashboard merges the PR via GitHub API                  │
│     └── Dashboard triggers Pull on all hosts                    │
│     └── Dashboard triggers Switch on all hosts                  │
│     └── Shows progress: "Deploying 3/9 hosts..."                │
│                                                                 │
│  4. (Optional) Full automation                                  │
│     └── Auto-merge after 1 hour (let CI pass)                   │
│     └── Auto-deploy to all hosts                                │
│     └── Notify you of success/failure                           │
└─────────────────────────────────────────────────────────────────┘
```

---

## Glossary

| Term           | Meaning                                                      |
| -------------- | ------------------------------------------------------------ |
| **flake.nix**  | Defines your Nix configuration and its inputs (dependencies) |
| **flake.lock** | Pins exact versions of all inputs (like package-lock.json)   |
| **nixpkgs**    | The main Nix package repository (80,000+ packages)           |
| **generation** | A commit hash representing a deployed configuration version  |
| **switch**     | Apply a new system configuration (`nixos-rebuild switch`)    |
| **PR**         | Pull Request on GitHub                                       |

---

## Example: What Happens When You Update

**Before update (flake.lock):**

```json
{
  "nixpkgs": {
    "locked": {
      "rev": "abc123...",
      "lastModified": 1702234567 // Dec 10, 2025
    }
  }
}
```

**After `nix flake update` (new flake.lock):**

```json
{
  "nixpkgs": {
    "locked": {
      "rev": "def456...",
      "lastModified": 1702839367 // Dec 17, 2025
    }
  }
}
```

This means all packages will be built from the newer nixpkgs snapshot, potentially with:

- Security fixes
- New package versions
- Bug fixes
- Breaking changes (rare but possible)

---

## The Fundamental Question: Who Runs the Update?

**Someone has to run `nix flake update`.** This command fetches the latest versions of all inputs and writes them to `flake.lock`.

There are two options for WHO does this:

| Option            | Who runs `nix flake update`? | How it gets to all hosts            |
| ----------------- | ---------------------------- | ----------------------------------- |
| **GitHub-driven** | GitHub Action (CI)           | Creates PR → merge → hosts pull     |
| **Host-driven**   | One of your hosts            | Commits → pushes → other hosts pull |

### Option C: GitHub-Driven (Current Plan - P4300)

```
GitHub Action  ──→  PR  ──→  NixFleet detects  ──→  Merge & Deploy
```

**Why this approach:**

- It's the existing workflow (from pbek/hokage) — we change as little as possible
- GitHub Actions is already set up and running weekly
- CI can run checks before you merge
- Clear audit trail in git history

**P4300 just makes it smoother** — instead of manually reviewing PRs on GitHub, NixFleet shows "update available" and offers one-click merge + deploy.

### Option A: Host-Driven (Future Feature)

```
Dashboard "Update Inputs" button  ──→  One host runs update  ──→  Push  ──→  Deploy all
```

**Why this might be added later:**

- Simpler for users who don't want/need the GitHub PR workflow
- Fewer moving parts
- Works without GitHub Actions configured

**This will be a toggle in the Settings page** — choose your preferred update strategy.

---

## Agent Version Tracking

The NixFleet agent has its own versioning, **separate** from your `flake.lock`:

```
┌──────────────────────────────────────────────────────────────────┐
│  flake.lock   = which version of nixpkgs/home-manager you use   │
│  agent        = which version of the NixFleet agent is running  │
└──────────────────────────────────────────────────────────────────┘
```

### How It Works

1. **Dashboard knows** its own version (compiled in at build time)
2. **Agents report** their version in every heartbeat
3. **If they differ** → Agent is outdated

### Visual Indicator

When an agent is outdated, the **Lock compartment indicator turns red**:

```
┌──────────┬──────────┬──────────┐
│   Git    │   Lock   │  System  │
│   🟢     │   🔴     │   🟢     │  ← Red Lock = agent outdated
└──────────┴──────────┴──────────┘
```

The Lock compartment's tooltip shows detailed agent version info:

```
✗ Agent needs update

Installed: 2.0.0
Expected:  2.1.0

Run 'switch' to update the agent.

─────────────────────────

✓ Dependencies up to date

flake.lock matches the latest
available package versions.
```

### Why on Lock? (Not a 4th Compartment)

The agent version is tied to your `flake.lock` because:

- The agent is defined as a Nix input in `flake.nix`
- Updating `flake.lock` (via `nix flake update`) bumps the agent input
- Running `switch` deploys the new agent

So **updating the Lock** → **updates the Agent**. They're conceptually linked.

### macOS-Specific Issue

On macOS, even after a successful `switch`, the agent may still report the old version. This is because launchd doesn't automatically reload the updated plist.

**Fix**: After switch on macOS, restart the agent:

- Dashboard: **⋮** → **Restart Agent**
- CLI: `launchctl kickstart -k gui/$(id -u)/com.nixfleet.agent`

See [P1100](../+pm/backlog/P1100-macos-agent-update-bug.md) for details on this issue.

### Potential Issues: Browser Caching

⚠️ The dashboard's version comes from its compiled code. If your browser caches an old dashboard version, it might show false positives ("agent outdated" when it isn't).

**If you see unexpected red Lock indicators:**

1. Hard refresh: `Cmd+Shift+R` (Mac) or `Ctrl+Shift+R` (Windows/Linux)
2. Clear browser cache for the dashboard URL
3. Verify the dashboard container restarted after deploy

---

## Summary: Update Modes

NixFleet supports three update modes for flexibility:

| Mode                   | Scope       | Control Level  | Best For                |
| ---------------------- | ----------- | -------------- | ----------------------- |
| **Manual per-step**    | Per host    | Full manual    | Debugging, testing      |
| **Per-host automatic** | Single host | Semi-automatic | Individual host updates |
| **Fleet-wide**         | All hosts   | Automated      | Regular maintenance     |

See [UPDATE-ARCHITECTURE.md](./UPDATE-ARCHITECTURE.md) for complete documentation of the update flow.

---

## Related Documentation

- [UPDATE-ARCHITECTURE.md](./UPDATE-ARCHITECTURE.md) — Complete update flow and troubleshooting
- [BUILD-DEPLOY.md](./BUILD-DEPLOY.md) — How components are built and deployed
- [P4300](../+pm/backlog/P4300-automated-flake-updates.md) — Automated flake updates backlog item
- [P1100](../+pm/backlog/P1100-macos-agent-update-bug.md) — macOS agent update bug
