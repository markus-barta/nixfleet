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

## The Ideal Workflow (P5300 Goal)

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
