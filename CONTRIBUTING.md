# Git Workflow & Branching Strategy

We use **Git Flow** for this project.

## 1. Main Branches
*   **`main`**: Production code. Always stable.
*   **`develop`**: Development code. Merge PRs here.

## 2. Naming Conventions

### 🆕 Features
*   **Pattern**: `feature/<name-of-feature>`
*   **Source**: `develop`
*   **Ref**:
    *   `feature/user-auth`
    *   `feature/video-transcoding`
    *   `feature/add-docker-compose`

### 🐛 Bug Fixes
*   **Pattern**: `bugfix/<what-is-broken>`
*   **Source**: `develop`
*   **Ref**:
    *   `bugfix/fix-login-500`
    *   `bugfix/kafka-retry-logic`

### 🔥 Hotfixes (Prod Critical)
*   **Pattern**: `hotfix/<critical-issue>`
*   **Source**: `main`
*   **Ref**:
    *   `hotfix/security-patch-v1`
    *   `hotfix/payment-gateway-fail`

### 📦 Releases
*   **Pattern**: `release/v<x.y.z>`
*   **Source**: `develop`
*   **Ref**:
    *   `release/v1.0.0`

## 3. Workflow
1.  Checkout `develop`: `git checkout develop`
2.  Pull latest: `git pull origin develop`
3.  Create branch: `git checkout -b feature/my-cool-feature`
4.  Work & Commit
5.  Push: `git push origin feature/my-cool-feature`
6.  Create Pull Request (PR) to `develop`
