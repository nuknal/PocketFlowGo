# PocketFlowGo UI

The web interface for PocketFlowGo, built with React, Vite, and TypeScript. This UI provides a visual dashboard to manage flows, tasks, and workers, including a powerful flow visualizer.

## Tech Stack

- **Framework:** [React 19](https://react.dev/)
- **Build Tool:** [Vite](https://vitejs.dev/)
- **Language:** [TypeScript](https://www.typescriptlang.org/)
- **Styling:** [Tailwind CSS](https://tailwindcss.com/)
- **UI Components:** [shadcn/ui](https://ui.shadcn.com/) (based on Radix UI)
- **Flow Visualization:** [React Flow](https://reactflow.dev/) (@xyflow/react)
- **Icons:** [Lucide React](https://lucide.dev/)
- **Routing:** [React Router](https://reactrouter.com/)

## Project Structure

```
ui/
├── src/
│   ├── assets/         # Static assets
│   ├── components/     # Reusable components
│   │   ├── flow/       # Custom React Flow nodes and visualizer
│   │   ├── layout/     # App layout (Sidebar, etc.)
│   │   └── ui/         # Base UI components (shadcn/ui)
│   ├── lib/            # Utilities and API client
│   ├── pages/          # Application pages
│   └── App.tsx         # Main application entry
├── embed.go            # Go embed directive for bundling UI
└── package.json        # Dependencies and scripts
```

## Getting Started

### Prerequisites

- Node.js (v18 or later recommended)
- npm or yarn or pnpm

### Installation

1. Navigate to the `ui` directory:
   ```bash
   cd ui
   ```

2. Install dependencies:
   ```bash
   npm install
   ```

### Development

Start the development server:

```bash
npm run dev
```

The application will be available at `http://localhost:5173` (or the port shown in the terminal).

> **Note:** Ensure the PocketFlowGo backend server is running to fetch real data. You may need to configure the API endpoint in `src/lib/api.ts` if it differs from the default.

### Build

Build the application for production:

```bash
npm run build
```

This will generate a `dist` folder with the compiled assets.

### Embedding in Go

This project is designed to be embedded into the PocketFlowGo Go binary. The `embed.go` file facilitates this integration. After building the UI, the Go build process will include the contents of the `dist` directory.

## Features

- **Dashboard:** Overview of system status.
- **Flow Management:** Visualize, create, and manage workflows.
- **Task Tracking:** Monitor task execution and history.
- **Worker Management:** View connected workers and their status.
- **Dark Mode:** Built-in theme switching support.
