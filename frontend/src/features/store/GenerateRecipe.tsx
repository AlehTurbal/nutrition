import { useState } from "react";
import { useGenerateRecipe } from "../../api/llm";
import { Button, Card, ErrorBox, Field } from "../../components/ui";
import { fmt, slotLabel } from "../../lib/labels";

export default function GenerateRecipe() {
  const gen = useGenerateRecipe();
  const [description, setDescription] = useState("");
  const [servings, setServings] = useState(1);

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    gen.mutate({ description: description.trim(), meal_types: [], servings });
  };

  return (
    <Card title="Генерация рецепта (LLM)">
      <form onSubmit={submit} className="space-y-3">
        <Field
          label="Опишите блюдо"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          placeholder="например: лёгкий салат с курицей"
          required
        />
        <Field
          label="Порций"
          type="number"
          min={1}
          value={servings}
          onChange={(e) => setServings(Number(e.target.value))}
        />
        {gen.isError && <ErrorBox error={gen.error} />}
        <Button type="submit" disabled={gen.isPending}>
          {gen.isPending ? "Генерация…" : "Сгенерировать"}
        </Button>
      </form>

      {gen.data && (
        <div className="mt-4 space-y-2 border-t border-slate-100 pt-4">
          <h3 className="font-semibold text-slate-700">{gen.data.recipe.name}</h3>
          <div className="flex flex-wrap gap-1">
            {gen.data.recipe.meal_types.map((m) => (
              <span key={m} className="rounded-full bg-brand-50 px-2 py-0.5 text-xs text-brand-700">
                {slotLabel(m)}
              </span>
            ))}
          </div>
          <p className="text-sm text-slate-500">{gen.data.recipe.instructions}</p>
          <ul className="text-sm">
            {gen.data.ingredients.map((ing, i) => (
              <li key={i} className="flex justify-between border-b border-slate-50 py-1">
                <span>
                  {ing.name}{" "}
                  {ing.product_id == null && (
                    <span className="text-amber-600" title="Нет такого продукта">
                      (создать продукт)
                    </span>
                  )}
                </span>
                <span className="text-slate-500">{fmt(ing.grams)} г</span>
              </li>
            ))}
          </ul>
          <p className="text-xs text-slate-400">
            Чтобы сохранить рецепт, создайте недостающие продукты на вкладке «Продукты»,
            затем добавьте рецепт вручную. (Автосохранение появится позже.)
          </p>
        </div>
      )}
    </Card>
  );
}
