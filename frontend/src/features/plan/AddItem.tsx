import { useState } from "react";
import { useAddItem } from "../../api/plans";
import type { Recipe } from "../../api/types";
import { Button } from "../../components/ui";

export default function AddItem({
  planId,
  day,
  slot,
  recipes,
  onDone,
}: {
  planId: number;
  day: string;
  slot: string;
  recipes: Recipe[];
  onDone: () => void;
}) {
  const add = useAddItem(planId);
  const [recipeId, setRecipeId] = useState<number>(recipes[0]?.id ?? 0);
  const [servings, setServings] = useState(1);

  if (recipes.length === 0) {
    return (
      <div className="rounded border border-slate-200 p-2 text-xs text-slate-400">
        Нет рецептов.{" "}
        <button onClick={onDone} className="text-brand-500">
          закрыть
        </button>
      </div>
    );
  }

  const submit = () =>
    add.mutate(
      { day_date: day, meal_slot: slot, recipe_id: recipeId, servings },
      { onSuccess: onDone },
    );

  return (
    <div className="space-y-1 rounded border border-slate-200 p-2">
      <select
        value={recipeId}
        onChange={(e) => setRecipeId(Number(e.target.value))}
        className="w-full rounded border border-slate-300 px-1 py-1 text-xs"
      >
        {recipes.map((r) => (
          <option key={r.id} value={r.id}>{r.name}</option>
        ))}
      </select>
      <div className="flex items-center gap-1">
        <input
          type="number"
          min={0.5}
          step={0.5}
          value={servings}
          onChange={(e) => setServings(Number(e.target.value))}
          className="w-16 rounded border border-slate-300 px-1 py-1 text-xs"
        />
        <span className="text-xs text-slate-400">порц.</span>
        <Button
          type="button"
          onClick={submit}
          disabled={add.isPending}
          className="ml-auto px-2 py-0.5 text-xs"
        >
          ОК
        </Button>
        <Button
          type="button"
          variant="ghost"
          onClick={onDone}
          className="px-2 py-0.5 text-xs"
        >
          ✕
        </Button>
      </div>
    </div>
  );
}
