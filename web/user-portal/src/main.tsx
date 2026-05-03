import React from 'react';
import ReactDOM from 'react-dom/client';
import App from './App';
import './index.css';
import { loadBranding, initDarkMode } from './branding';

// Apply dark mode immediately (before paint) to prevent a flash of light theme.
initDarkMode();

loadBranding().then(() => {
  ReactDOM.createRoot(document.getElementById('root')!).render(
    <React.StrictMode>
      <App />
    </React.StrictMode>,
  );
});
