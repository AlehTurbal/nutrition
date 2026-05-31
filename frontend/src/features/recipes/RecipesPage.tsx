import { useState } from "react";
import {
  useCreateRecipe,
  useDeleteRecipe,
  useRecipe,
  useRecipes,
  useUpdateRecipe,
} from "../../api/recipes";
import type { RecipeInput } from "../../api/types";
import { Button, Card, Empty, ErrorBox, Spinner } from "../../components/ui";
import { fmt, slotLabel } from "../../lib/labels";
import RecipeForm from "./RecipeForm";

export default function RecipesPage() {
  const { data, isLoading, isError, error } = useRecipes();
  const create = useCreateRecipe();
  const update = useUpdateRecipe();
  const del = useDeleteRecipe();
  const [adding, setAdding] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const editing = useRecipe(editingId);

  const submit = (input: RecipeInput) => {
    if (editingId) {
      update.mutate(
        { id: editingId, input },
        { onSuccess: () => setEditingId(null) },
      );
    } else {
      create.mutate(input, { onSuccess: () => setAdding(false) });
    }
  };

  const showForm = adding || editingId != null;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold text-slate-800">Рецепты</h1>
        {!showForm && <Button onClick={() => setAdding(true)}>+ Рецепт</Button>}
      </div>

      {showForm &&
        (editingId != null && editing.isLoading ? (
          <Card><Spinner /></Card>
        ) : (
          <Card title={editingId ? "Редактирование рецепта" : "Новый рецепт"}>
            <RecipeForm
              initial={editing.data}
              pending={create.isPending || update.isPending}
              error={create.error ?? update.error}
              onSubmit={submit}
              onCancel={() => {
                setAdding(false);
                setEditingId(null);
              }}
            />
          </Card>
        ))}

      {isLoading ? (
        <Spinner />
      ) : isError ? (
        <ErrorBox error={error} />
      ) : !data || data.length === 0 ? (
        <Card><Empty>Пока нет рецептов</Empty></Card>
      ) : (
        <div className="grid gap-4 md:grid-cols-2">
          {data.map((r) => (
            <Card
              key={r.id}
              title={r.name}
              actions={
                <div className="flex gap-2">
                  <Button variant="ghost" onClick={() => setEditingId(r.id)}>✎</Button>
                  <Button
                    variant="danger"
                    onClick={() => confirm(`Удалить «${r.name}»?`) && del.mutate(r.id)}
                  >
                    ✕
                  </Button>
                </div>
              }
            >
              <div className="space-y-2 text-sm text-slate-600">
                <div>Порций: {fmt(r.servings)}</div>
                <div className="flex flex-wrap gap-1">
                  {r.meal_types.length === 0 ? (
                    <span className="text-slate-400">любой приём</span>
                  ) : (
                    r.meal_types.map((m) => (
                      <span
                        key={m}
                        className="rounded-full bg-brand-50 px-2 py-0.5 text-xs text-brand-700"
                      >
                        {slotLabel(m)}
                      </span>
                    ))
                  )}
                </div>
                {r.instructions && (
                  <p className="text-slate-500">{r.instructions}</p>
                )}
              </div>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
