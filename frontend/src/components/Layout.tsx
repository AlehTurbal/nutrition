import type { ReactNode } from "react";
import { NavLink } from "react-router-dom";
import { clearToken } from "../lib/auth";

const nav = [
  { to: "/", label: "Дашборд", end: true },
  { to: "/products", label: "Продукты" },
  { to: "/recipes", label: "Рецепты" },
  { to: "/plan", label: "План питания" },
  { to: "/shopping", label: "Покупки" },
  { to: "/store", label: "Магазин" },
];

export default function Layout({ children }: { children: ReactNode }) {
  return (
    <div className="flex h-full flex-col">
      <header className="flex items-center justify-between border-b border-slate-200 bg-white px-5 py-3">
        <div className="flex items-center gap-2 font-semibold text-brand-700">
          <span className="text-xl">🥗</span> Питание
        </div>
        <button
          onClick={() => clearToken()}
          className="text-sm text-slate-500 hover:text-slate-800"
        >
          Выйти
        </button>
      </header>

      <div className="flex min-h-0 flex-1">
        <nav className="w-48 shrink-0 border-r border-slate-200 bg-white p-3">
          {nav.map((n) => (
            <NavLink
              key={n.to}
              to={n.to}
              end={n.end}
              className={({ isActive }) =>
                `mb-1 block rounded-lg px-3 py-2 text-sm font-medium ${
                  isActive
                    ? "bg-brand-50 text-brand-700"
                    : "text-slate-600 hover:bg-slate-100"
                }`
              }
            >
              {n.label}
            </NavLink>
          ))}
        </nav>

        <main className="min-w-0 flex-1 overflow-y-auto p-6">{children}</main>

        <aside className="hidden w-72 shrink-0 flex-col border-l border-slate-200 bg-white lg:flex">
          <div className="border-b border-slate-100 px-4 py-3 font-semibold text-slate-700">
            Ассистент
          </div>
          <div className="flex flex-1 flex-col items-center justify-center gap-2 p-6 text-center text-sm text-slate-400">
            <span className="text-3xl">💬</span>
            <p>Чат-ассистент появится в Фазе 5.</p>
            <p>Здесь можно будет создавать блюда и меню обычным языком.</p>
          </div>
        </aside>
      </div>
    </div>
  );
}
