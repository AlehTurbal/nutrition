import { useState } from "react";
import { useProducts } from "../../api/products";
import type { MealSlot, RecipeInput, RecipeResponse } from "../../api/types";
import { Button, ErrorBox, Field, Spinner } from "../../components/ui";
import { fmt, mealSlots } from "../../lib/labels";

interface Row {
  product_id: number;
  grams: string;
}

export default function RecipeForm({
  initial,
  pending,
  error,
  onSubmit,
  onCancel,
}: {
  initial?: RecipeResponse;
  pending: boolean;
  error: unknown;
  onSubmit: (input: RecipeInput) => void;
  onCancel: () => void;
}) {
  const products = useProducts();
  const r = initial?.recipe;

  const [name, setName] = useState(r?.name ?? "");
  const [servings, setServings] = useState(r?.servings ?? 1);
  const [instructions, setInstructions] = useState(r?.instructions ?? "");
  const [slots, setSlots] = useState<MealSlot[]>(r?.meal_types ?? []);
  const [rows, setRows] = useState<Row[]>(
    r?.ingredients?.map((i) => ({
      product_id: i.product_id,
      grams: String(i.grams),
    })) ?? [],
  );

  const toggleSlot = (s: MealSlot) =>
    setSlots((cur) => (cur.includes(s) ? cur.filter((x) => x !== s) : [...cur, s]));

  const addRow = () =>
    setRows((cur) => [
      ...cur,
      { product_id: products.data?.[0]?.id ?? 0, grams: "100" },
    ]);

  const setRow = (i: number, patch: Partial<Row>) =>
    setRows((cur) => cur.map((row, idx) => (idx === i ? { ...row, ...patch } : row)));

  const removeRow = (i: number) =>
    setRows((cur) => cur.filter((_, idx) => idx !== i));

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    onSubmit({
      name: name.trim(),
      servings,
      instructions: instructions.trim(),
      meal_types: slots,
      ingredients: rows
        .filter((row) => row.product_id > 0 && Number(row.grams) > 0)
        .map((row) => ({ product_id: row.product_id, grams: Number(row.grams) })),
    });
  };

  if (products.isLoading) return <Spinner />;
  const hasProducts = (products.data?.length ?? 0) > 0;

  return (
    <form onSubmit={submit} className="space-y-4">
      <div className="grid gap-3 sm:grid-cols-2">
        <Field
          label="Название"
          required
          value={name}
          onChange={(e) => setName(e.target.value)}
        />
        <Field
          label="Порций"
          type="number"
          min={1}
          value={servings}
          onChange={(e) => setServings(Number(e.target.value))}
        />
      </div>

      <div>
        <span className="mb-1 block text-sm font-medium text-slate-600">
          Подходит для приёмов
        </span>
        <div className="flex flex-wrap gap-2">
          {mealSlots.map((s) => {
            const on = slots.includes(s.value);
            return (
              <button
                key={s.value}
                type="button"
                onClick={() => toggleSlot(s.value)}
                className={`rounded-full border px-3 py-1 text-sm ${
                  on
                    ? "border-brand-500 bg-brand-50 text-brand-700"
                    : "border-slate-300 text-slate-500"
                }`}
              >
                {s.label}
              </button>
            );
          })}
        </div>
      </div>

      <div>
        <div className="mb-2 flex items-center justify-between">
          <span className="text-sm font-medium text-slate-600">Ингредиенты</span>
          <Button type="button" variant="ghost" onClick={addRow} disabled={!hasProducts}>
            + Ингредиент
          </Button>
        </div>
        {!hasProducts && (
          <p className="mb-2 text-sm text-slate-400">
            Сначала добавьте продукты на вкладке «Продукты».
          </p>
        )}
        <div className="space-y-2">
          {rows.map((row, i) => (
            <div key={i} className="flex items-center gap-2">
              <select
                value={row.product_id}
                onChange={(e) => setRow(i, { product_id: Number(e.target.value) })}
                className="flex-1 rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-sm"
              >
                {products.data!.map((p) => (
                  <option key={p.id} value={p.id}>{p.name}</option>
                ))}
              </select>
              <input
                type="number"
                min={1}
                value={row.grams}
                onChange={(e) => setRow(i, { grams: e.target.value })}
                className="w-24 rounded-lg border border-slate-300 px-3 py-1.5 text-sm"
              />
              <span className="text-sm text-slate-400">г</span>
              <Button type="button" variant="danger" onClick={() => removeRow(i)}>
                ✕
              </Button>
            </div>
          ))}
        </div>
      </div>

      <label className="block text-sm">
        <span className="mb-1 block font-medium text-slate-600">Инструкция</span>
        <textarea
          value={instructions}
          onChange={(e) => setInstructions(e.target.value)}
          rows={3}
          className="w-full rounded-lg border border-slate-300 px-3 py-1.5 outline-none focus:border-brand-500 focus:ring-1 focus:ring-brand-500"
        />
      </label>

      {initial && (
        <div className="rounded-lg bg-slate-50 p-3 text-sm text-slate-600">
          Итого: <b>{fmt(initial.macros.kcal)}</b> ккal · Б {fmt(initial.macros.protein)} ·
          Ж {fmt(initial.macros.fat)} · У {fmt(initial.macros.carbs)}
          {!initial.macros.complete && (
            <span className="ml-2 text-amber-600">⚠ неполные данные</span>
          )}
        </div>
      )}

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
