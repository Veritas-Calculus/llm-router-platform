import React from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import { ApolloProvider } from '@apollo/client/react';
import * as Sentry from '@sentry/react';
import { apolloClient } from './lib/graphql/client';
import ErrorBoundary from './components/ErrorBoundary';
import ThemedToaster from './components/ThemedToaster';
import { getRuntimeConfig } from './lib/runtime-config';
import App from './App';
import './index.css';

// Initialize Sentry error tracking.
//
// Reads the DSN from runtime config (window.__RUNTIME_CONFIG__) so the same
// production image can be promoted dev → staging → prod without rebuild
// (audit H-08). Falls back to import.meta.env.VITE_SENTRY_DSN so `npm run
// dev` (no nginx envsubst) keeps working unchanged.
//
// Treats an empty / unset DSN as "Sentry disabled" and skips `Sentry.init`
// entirely — without this guard the bundle still spins up the Replay
// worker (CSP-rejected blob: Worker, see audit H-02) and every outbound
// fetch leaks `baggage` / `sentry-trace` headers even though nothing is
// being collected (audit H-03). Whitespace placeholders are trimmed by
// `getRuntimeConfig`, defending against build pipelines that set the arg
// to a whitespace string.
const sentryDSN = getRuntimeConfig('SENTRY_DSN');
const sentryEnvironment = getRuntimeConfig('SENTRY_ENVIRONMENT') || 'production';
if (sentryDSN) {
  Sentry.init({
    dsn: sentryDSN,
    environment: sentryEnvironment,
    integrations: [
      Sentry.browserTracingIntegration(),
      Sentry.replayIntegration(),
    ],
    tracesSampleRate: 0.2,
    replaysSessionSampleRate: 0.1,
    replaysOnErrorSampleRate: 1.0,
    // Restrict trace-propagation headers (`baggage`, `sentry-trace`) to
    // first-party origins. Without this, every fetch — including upstream
    // `/v1/chat/completions` calls — adds ~300 bytes of unused Sentry
    // headers (audit findings H-03 / I-02). `/^\//` matches same-origin
    // requests (relative URLs); the localhost regex covers dev-only direct
    // backend hits. Add production hostnames here if/when needed.
    //
    // I-02 re-verify (Round 7): confirmed this list is the ONLY allow-list
    // for outbound trace headers. browserTracingIntegration() reads it on
    // every fetch wrap and skips header injection for non-matching URLs;
    // there is no separate `tracingOrigins` legacy setting active.
    tracePropagationTargets: [/^\//, /^https?:\/\/localhost(?::\d+)?(?:\/|$)/],
  });
}

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ErrorBoundary>
      <ApolloProvider client={apolloClient}>
        <BrowserRouter>
          <App />
          <ThemedToaster />
        </BrowserRouter>
      </ApolloProvider>
    </ErrorBoundary>
  </React.StrictMode>
);

