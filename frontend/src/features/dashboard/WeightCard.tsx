import { useState } from "react";
import { useAddWeight, useWeights } from "../../api/profile";
import { Button, Card, Empty, ErrorBox, Field, Spinner } from "../../components/ui";
import { fmt } from "../../lib/labels";

export default function WeightCard() {
  const { data, isLoading, isError, error } = useWeights();
  const add = useAddWeight();
  const [weight, setWeight] = useState("");

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    const w = Number(weight);
    if (w > 0) add.mutate(w, { onSuccess: () => setWeight("") });
  };

  return (
    <Card title="Вес">
      <form onSubmit={submit} className="mb-4 flex items-end gap-3">
        <div className="flex-1">
          <Field
            label="Новый вес, кг"
            type="number"
            step="0.1"
            min={1}
            value={weight}
            onChange={(e) => setWeight(e.target.value)}
          />
        </div>
        <Button type="submit" disabled={add.isPending}>
          Добавить
        </Button>
      </form>
      {add.isError && <ErrorBox error={add.error} />}

      {isLoading ? (
        <Spinner />
      ) : isError ? (
        <ErrorBox error={error} />
      ) : !data || data.length === 0 ? (
        <Empty>Пока нет записей веса</Empty>
      ) : (
        <ul className="max-h-56 space-y-1 overflow-y-auto text-sm">
          {data.map((w) => (
            <li
              key={w.id}
              className="flex justify-between border-b border-slate-50 py-1"
            >
              <span className="font-medium text-slate-700">
                {fmt(w.weight_kg)} кг
              </span>
              <span className="text-slate-400">
                {new Date(w.recorded_at).toLocaleDateString("ru-RU")}
              </span>
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}
