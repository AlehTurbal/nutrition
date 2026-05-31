import { useEffect, useState } from "react";
import { useProfile, useSaveProfile } from "../../api/profile";
import type { MealSlot, ProfileInput } from "../../api/types";
import { Button, Card, ErrorBox, Field, Select, Spinner } from "../../components/ui";
import { activities, goals, mealSlots, sexes } from "../../lib/labels";

const empty: ProfileInput = {
  sex: "male",
  height_cm: 175,
  age: 30,
  activity_level: "moderate",
  goal: "maintain",
  protein_per_kg: 0,
  fat_pct: 0,
  meal_slots: ["breakfast", "lunch", "dinner"],
};

export default function ProfileCard() {
  const { data, isLoading, isError, error } = useProfile();
  const save = useSaveProfile();
  const [form, setForm] = useState<ProfileInput>(empty);

  useEffect(() => {
    if (data) {
      setForm({
        sex: data.sex,
        height_cm: data.height_cm,
        age: data.age,
        activity_level: data.activity_level,
        goal: data.goal,
        protein_per_kg: data.protein_per_kg,
        fat_pct: data.fat_pct,
        meal_slots: data.meal_slots?.length
          ? data.meal_slots
          : empty.meal_slots,
      });
    }
  }, [data]);

  if (isLoading) return <Card title="Профиль"><Spinner /></Card>;
  if (isError) return <Card title="Профиль"><ErrorBox error={error} /></Card>;

  const set = (patch: Partial<ProfileInput>) =>
    setForm((f) => ({ ...f, ...patch }));

  const toggleSlot = (slot: MealSlot) =>
    setForm((f) => ({
      ...f,
      meal_slots: f.meal_slots.includes(slot)
        ? f.meal_slots.filter((s) => s !== slot)
        : [...f.meal_slots, slot],
    }));

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    save.mutate(form);
  };

  return (
    <Card title="Профиль">
      <form onSubmit={submit} className="space-y-4">
        <div className="grid grid-cols-2 gap-3">
          <Select
            label="Пол"
            value={form.sex}
            onChange={(e) => set({ sex: e.target.value as ProfileInput["sex"] })}
          >
            {sexes.map((s) => (
              <option key={s.value} value={s.value}>{s.label}</option>
            ))}
          </Select>
          <Field
            label="Возраст"
            type="number"
            min={1}
            value={form.age}
            onChange={(e) => set({ age: Number(e.target.value) })}
          />
          <Field
            label="Рост, см"
            type="number"
            min={1}
            value={form.height_cm}
            onChange={(e) => set({ height_cm: Number(e.target.value) })}
          />
          <Select
            label="Активность"
            value={form.activity_level}
            onChange={(e) =>
              set({ activity_level: e.target.value as ProfileInput["activity_level"] })
            }
          >
            {activities.map((a) => (
              <option key={a.value} value={a.value}>{a.label}</option>
            ))}
          </Select>
          <Select
            label="Цель"
            value={form.goal}
            onChange={(e) => set({ goal: e.target.value as ProfileInput["goal"] })}
          >
            {goals.map((g) => (
              <option key={g.value} value={g.value}>{g.label}</option>
            ))}
          </Select>
        </div>

        <details className="text-sm text-slate-500">
          <summary className="cursor-pointer select-none">
            Тонкая настройка макросов
          </summary>
          <div className="mt-3 grid grid-cols-2 gap-3">
            <Field
              label="Белок, г/кг (0 = по умолч.)"
              type="number"
              step="0.1"
              min={0}
              value={form.protein_per_kg}
              onChange={(e) => set({ protein_per_kg: Number(e.target.value) })}
            />
            <Field
              label="Жиры, доля (0 = по умолч.)"
              type="number"
              step="0.05"
              min={0}
              max={1}
              value={form.fat_pct}
              onChange={(e) => set({ fat_pct: Number(e.target.value) })}
            />
          </div>
        </details>

        <div>
          <span className="mb-1 block text-sm font-medium text-slate-600">
            Приёмы пищи в день ({form.meal_slots.length})
          </span>
          <div className="flex flex-wrap gap-2">
            {mealSlots.map((s) => {
              const on = form.meal_slots.includes(s.value);
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

        {save.isError && <ErrorBox error={save.error} />}
        <div className="flex items-center gap-3">
          <Button type="submit" disabled={save.isPending}>
            {save.isPending ? "Сохранение…" : "Сохранить профиль"}
          </Button>
          {save.isSuccess && (
            <span className="text-sm text-brand-600">Сохранено ✓</span>
          )}
        </div>
      </form>
    </Card>
  );
}
