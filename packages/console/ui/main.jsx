import React, { Suspense, lazy, useEffect, useMemo, useState } from "react";
import { createRoot } from "react-dom/client";
import "./styles.css";

const HomePage = lazy(() => import("./pages/HomePage.jsx"));
const LoginPage = lazy(() => import("./pages/LoginPage.jsx"));
const ConsolePage = lazy(() => import("./pages/ConsolePage.jsx"));

function currentRoute() {
  const route = window.location.hash.replace(/^#\/?/, "");
  return route || "home";
}

function App() {
  const [route, setRoute] = useState(currentRoute());
  const [session, setSession] = useState(null);
  const [authChecked, setAuthChecked] = useState(false);

  useEffect(() => {
    const onHashChange = () => setRoute(currentRoute());
    window.addEventListener("hashchange", onHashChange);
    return () => window.removeEventListener("hashchange", onHashChange);
  }, []);

  useEffect(() => {
    let cancelled = false;
    fetch("/api/auth/me")
      .then(async (response) => {
        if (!response.ok) return null;
        return response.json();
      })
      .then((payload) => {
        if (cancelled) return;
        if (payload?.user) setSession(payload);
      })
      .finally(() => {
        if (!cancelled) setAuthChecked(true);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const page = useMemo(() => {
    if (route === "login") {
      return <LoginPage onLogin={(payload) => {
        setSession(payload);
        window.location.hash = "console";
      }} />;
    }
    if (route === "console") {
      return session
        ? <ConsolePage session={session} onLogout={() => setSession(null)} />
        : <LoginPage onLogin={(payload) => {
          setSession(payload);
          window.location.hash = "console";
        }} />;
    }
    return <HomePage session={session} />;
  }, [route, session]);

  if (!authChecked) return <div className="loading">正在加载 OPL Cloud...</div>;

  return (
    <Suspense fallback={<div className="loading">正在加载 OPL Cloud...</div>}>
      {page}
    </Suspense>
  );
}

createRoot(document.getElementById("root")).render(<App />);
