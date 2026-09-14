# JustPing

> A modern, distributed network latency and ICMP quality monitoring platform built with Go, PostgreSQL, React, Vite, and Tailwind CSS.

[![Build & Release](https://github.com/guimc233/JustPing/actions/workflows/ci.yml/badge.svg)](https://github.com/guimc233/JustPing/actions/workflows/ci.yml)
[![Docker Image](https://img.shields.io/badge/Docker-ghcr.io%2Fguimc233%2Fjustping-blue?logo=docker)](https://github.com/guimc233/JustPing/pkgs/container/justping)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

---

## 🌟 Highlights

- **Modern Web Dashboard**: Built with Vite, React 19, Tailwind CSS v4, and shadcn/ui design patterns with dark mode.
- **Privacy First (Data Masking)**: Unauthenticated visitors can view ping latency, loss, and jitter without exposing sensitive server IP addresses or internal hostnames.
- **First-Run Setup Wizard**: On initial startup, an interactive wizard guides you through OAuth configuration and activates the **Superadmin** account.
- **Email Whitelist Authentication**:
  - Supports **GitHub OAuth**, **Google OAuth**, and **Generic OIDC** (compatible with LinuxDo, Keycloak, Casdoor, etc.).
  - Gravatar avatar support for non-GitHub OAuth providers based on verified email hash.
  - Granular access control: only emails approved in the whitelist are granted administrator privileges.
- **Distributed Go Agent**:
  - Precision ICMP probing: calculates packet loss, min/max/avg RTT, standard deviation, and **RFC 3550 Interarrival Jitter**.
  - WebSocket over TLS (WSS) persistent communication with automatic reconnection and offline ring buffering.
  - Wide architecture support: `amd64`, `386`, `arm64`, `armv7/v6/v5`, `riscv64`, `mips/mipsel/mips64/mips64el`, `ppc64le`, `s390x`.
- **Universal Linux Service Installer**:
  - One-line installation script (`install.sh`) with automatic architecture detection.
  - Native service integration across **systemd**, **OpenRC** (Alpine/Gentoo), **procd** (OpenWrt), **runit** (Void Linux), and legacy **SysVinit**.
- **Automated CI/CD**:
  - Multi-arch Docker image published to GitHub Container Registry (`ghcr.io/guimc233/justping`).
  - Automated binary builds for Linux and Windows across all target architectures.

---

## 🚀 Quick Start with Docker

The easiest way to run the JustPing Host server and PostgreSQL database:

```bash
# 1. Clone repository
git clone https://github.com/guimc233/JustPing.git
cd JustPing

# 2. Copy environment template
cp .env.example .env

# 3. Start services with Docker Compose
docker compose up -d
```

Visit `http://localhost:8080` in your browser. Complete the initial setup wizard to configure your OAuth app and become the Superadmin.

---

## 💻 Manual Binary Deployment

### Host Server

```bash
# Build frontend
cd web && npm install && npm run build && cd ..

# Run host
cd host
go build -o justping-host ./cmd/server
DATABASE_URL="postgres://user:password@localhost:5432/justping?sslmode=disable" ./justping-host
```

### Agent Deployment

In the Admin Panel (`/admin`), create an agent to generate your unique enrollment token and one-line command:

```bash
curl -fsSL https://ping.example.com/install.sh | sudo bash -s -- \
  --server https://ping.example.com \
  --token <YOUR_AGENT_TOKEN>
```

The script automatically detects your Linux init system (systemd, OpenRC, procd, runit, or SysVinit) and architecture, installs the binary, sets up system permissions, and starts the service.

---

## 📁 Repository Structure

```
JustPing/
├── .github/workflows/       # GitHub Actions CI (Multi-arch Docker & Binaries)
├── host/                    # Host Server (Go + PostgreSQL + WebSocket Hub)
│   ├── cmd/server/          # Main entrypoint (< 200 LOC per file)
│   └── internal/            # API routes, auth, database, models, ws hub
├── agent/                   # Agent Probe (Go + ICMP engine + WSS client)
│   ├── cmd/agent/           # Agent CLI & daemon
│   └── internal/            # ICMP engine, multi-init installer, WS loop
├── shared/protocol/         # Shared communication structures
├── web/                     # Frontend WebUI (Vite + React + Tailwind + shadcn)
├── scripts/install.sh       # One-line universal Linux agent installer
├── Dockerfile               # Multi-stage production container build
└── docker-compose.yml       # Production/development stack definition
```

---

## 📄 License

This project is open-sourced under the [MIT License](LICENSE).
