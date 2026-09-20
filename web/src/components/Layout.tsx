import React from 'react';
import { Link, NavLink } from 'react-router-dom';
import { ThemeSwitcher } from './ThemeSwitcher';

interface BreadcrumbItem {
  label: string;
  href?: string;
}

interface LayoutProps {
  breadcrumbs?: BreadcrumbItem[];
  children: React.ReactNode;
}

export const Layout: React.FC<LayoutProps> = ({ breadcrumbs, children }) => {
  return (
    <div className="container">
      <header className="navbar">
        <Link to="/" className="brand-link">
          🐾 Petstore
        </Link>
        <div className="navbar-right">
          <nav className="nav-links">
            <NavLink to="/" end>
              Directory
            </NavLink>
            <NavLink to="/docs">
              API Docs
            </NavLink>
            <a href="/openapi.yaml" target="_blank" rel="noopener noreferrer">
              OpenAPI Spec
            </a>
            <a href="https://localhost:8080/healthz" target="_blank" rel="noopener noreferrer">
              Health
            </a>
          </nav>
          <ThemeSwitcher />
        </div>
      </header>

      {breadcrumbs && breadcrumbs.length > 0 && (
        <nav className="breadcrumb" aria-label="Breadcrumb">
          {breadcrumbs.map((item, index) => {
            const isLast = index === breadcrumbs.length - 1;
            return (
              <React.Fragment key={index}>
                {index > 0 && <span className="breadcrumb-separator">/</span>}
                {isLast || !item.href ? (
                  <span>{item.label}</span>
                ) : (
                  <Link to={item.href}>{item.label}</Link>
                )}
              </React.Fragment>
            );
          })}
        </nav>
      )}

      <main>{children}</main>
    </div>
  );
};
