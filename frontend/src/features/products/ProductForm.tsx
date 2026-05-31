import { useState } from "react";
import type { Product, ProductInput } from "../../api/types";
import { Button, ErrorBox, Field } from "../../components/ui";

// Empty string => null (БЖУ unknown); a number => that value.
function numOrNull(s: string): number | null {
  return s.trim() === "" ? null : Number(s);
}
function str(v: number | null): string {
  return v == null ? "" : String(v);
}

export default function ProductForm({
  initial,
  pending,
  error,
  onSubmit,
  onCancel,
}: {
  initial?: Product;
  pending: boolean;
  error: unknown;
  onSubmit: (input: ProductInput) => void;
  onCancel: () => void;
}) {
  const [name, setName] = useState(initial?.name ?? "");
  const [category, setCategory] = useState(initial?.category ?? "");
  const [brand, setBrand] = useState(initial?.brand ?? "");
  const [kcal, setKcal] = useState(str(initial?.kcal100 ?? null));
  const [protein, setProtein] = useState(str(initial?.protein100 ?? null));
  const [fat, setFat] = useState(str(initial?.fat100 ?? null));
  const [carbs, setCarbs] = useState(str(initial?.carbs100 ?? null));

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    onSubmit({
      name: name.trim(),
      category: category.trim(),
      brand: brand.trim(),
      kcal100: numOrNull(kcal),
      protein100: numOrNull(protein),
      fat100: numOrNull(fat),
      carbs100: numOrNull(carbs),
    });
  };

  return (
    <form onSubmit={submit} className="space-y-4">
      <div className="grid gap-3 sm:grid-cols-3">
        <Field
          label="Название"
          required
          value={name}
          onChange={(e) => setName(e.target.value)}
        />
        <Field
          label="Категория"
          value={category}
          onChange={(e) => setCategory(e.target.value)}
        />
        <Field
          label="Бренд"
          value={brand}
          onChange={(e) => setBrand(e.target.value)}
        />
      </div>

      <div>
        <div className="mb-1 flex items-center justify-between">
          <span className="text-sm font-medium text-slate-600">
            БЖУ на 100 г (пусто = неизвестно)
          </span>
          <button
            type="button"
            disabled
            title="Появится в Фазе 4"
            className="cursor-not-allowed rounded-md bg-slate-100 px-2 py-1 text-xs text-slate-400"
          >
            ✨ Заполнить через LLM
          </button>
        </div>
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          <Field label="Ккал" type="number" step="0.1" min={0} value={kcal} onChange={(e) => setKcal(e.target.value)} />
          <Field label="Белки" type="number" step="0.1" min={0} value={protein} onChange={(e) => setProtein(e.target.value)} />
          <Field label="Жиры" type="number" step="0.1" min={0} value={fat} onChange={(e) => setFat(e.target.value)} />
          <Field label="Углеводы" type="number" step="0.1" min={0} value={carbs} onChange={(e) => setCarbs(e.target.value)} />
        </div>
      </div>

      {error != null && <ErrorBox error={error} />}
      <div className="flex gap-3">
        <Button type="submit" disabled={pending}>
          {pending ? "Сохранение…" : "Сохранить"}
        </Button>
        <Button type="button" variant="ghost" onClick={onCancel}>
          Отмена
        </Button>
      </div>
    </form>
  );
}
