# Frontend

SvelteKit web application for LUT visualization.

## Prerequisites

- Node.js 22+
- pnpm (recommended) or npm

## Installation

```bash
cd frontend
pnpm install
```

## Development

```bash
pnpm dev --port 7750
```

## Scripts

| Command | Description |
|---------|-------------|
| `pnpm dev` | Start development server |
| `pnpm build` | Build for production |
| `pnpm preview` | Preview production build |

## Tech Stack

- [SvelteKit](https://kit.svelte.dev/) - Framework
- [Svelte 5](https://svelte.dev/) - UI
- [Tailwind CSS 4](https://tailwindcss.com/) - Styling
- [Vite](https://vitejs.dev/) - Build tool
- [TypeScript](https://www.typescriptlang.org/) - Type safety

## Building for Production

```bash
pnpm build
```

Output will be in the `build/` directory.

## Configuration

The frontend expects the backend to be running at `http://localhost:7754`.
