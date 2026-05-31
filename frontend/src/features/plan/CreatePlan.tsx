import { useState } from "react";
import { useCreatePlan } from "../../api/plans";
import { Button, ErrorBox, Field } from "../../components/ui";
import { addDays, today } from "../../lib/dates";

export default function CreatePlan({
  onCreated,
  onCancel,
}: {
  onCreated: (id: number) => void;
  onCancel: () => void;
}) {
  const create = useCreatePlan();
  const [name, setName] = useState("Неделя");
  const [start, setStart] = useState(today());
  const [end, setEnd] = useState(addDays(today(), 6));

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    create.mutate(
      { name: name.trim(), start_date: start, end_date: end },
      { onSuccess: (p) => onCreated(p.id) },
    );
  };

  return (
    <form onSubmit={submit} className="space-y-4">
      <div className="grid gap-3 sm:grid-cols-3">
        <Field label="Название" value={name} onChange={(e) => setName(e.target.value)} />
        <Field
          label="Начало"
          type="date"
          value={start}
          onChange={(e) => setStart(e.target.value)}
        />
        <Field
          label="Конец"
          type="date"
          value={end}
          onChange={(e) => setEnd(e.target.value)}
        />
      </div>
      {create.isError && <ErrorBox error={create.error} />}
      <div className="flex gap-3">
        <Button type="submit" disabled={create.isPending}>
          {create.isPending ? "Создание…" : "Создать"}
        </Button>
        <Button type="button" variant="ghost" onClick={onCancel}>
          Отмена
        </Button>
      </div>
    </form>
  );
}
