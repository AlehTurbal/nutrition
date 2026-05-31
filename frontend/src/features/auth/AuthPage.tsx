import { useState } from "react";
import { useLogin, useRegister } from "../../api/auth";
import { Button, ErrorBox, Field } from "../../components/ui";

export default function AuthPage() {
  const [mode, setMode] = useState<"login" | "register">("login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");

  const login = useLogin();
  const register = useRegister();
  const active = mode === "login" ? login : register;

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    active.mutate({ email, password });
  };

  return (
    <div className="flex h-full items-center justify-center p-6">
      <div className="w-full max-w-sm rounded-2xl border border-slate-200 bg-white p-8 shadow-sm">
        <div className="mb-6 text-center">
          <div className="text-3xl">🥗</div>
          <h1 className="mt-2 text-xl font-semibold text-slate-800">
            Питание · БЖУ и меню
          </h1>
        </div>

        <div className="mb-5 flex rounded-lg bg-slate-100 p-1 text-sm font-medium">
          {(["login", "register"] as const).map((m) => (
            <button
              key={m}
              onClick={() => setMode(m)}
              className={`flex-1 rounded-md py-1.5 ${
                mode === m ? "bg-white shadow-sm text-brand-700" : "text-slate-500"
              }`}
            >
              {m === "login" ? "Вход" : "Регистрация"}
            </button>
          ))}
        </div>

        <form onSubmit={submit} className="space-y-4">
          <Field
            label="Email"
            type="email"
            required
            value={email}
            onChange={(e) => setEmail(e.target.value)}
          />
          <Field
            label="Пароль"
            type="password"
            required
            minLength={8}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
          {active.isError && <ErrorBox error={active.error} />}
          <Button type="submit" className="w-full" disabled={active.isPending}>
            {active.isPending
              ? "…"
              : mode === "login"
                ? "Войти"
                : "Зарегистрироваться"}
          </Button>
        </form>
        {mode === "register" && (
          <p className="mt-3 text-center text-xs text-slate-400">
            Пароль не короче 8 символов
          </p>
        )}
      </div>
    </div>
  );
}
