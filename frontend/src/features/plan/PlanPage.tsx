import { useState } from "react";
import { usePlans } from "../../api/plans";
import { Button, Card, Empty, ErrorBox, Spinner } from "../../components/ui";
import { shortDate } from "../../lib/dates";
import CreatePlan from "./CreatePlan";
import PlanGrid from "./PlanGrid";

export default function PlanPage() {
  const { data, isLoading, isError, error } = usePlans();
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [creating, setCreating] = useState(false);

  const selected = data?.find((p) => p.id === selectedId) ?? data?.[0] ?? null;

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold text-slate-800">План питания</h1>
        {!creating && <Button onClick={() => setCreating(true)}>+ План</Button>}
      </div>

      {creating && (
        <Card title="Новый план">
          <CreatePlan
            onCreated={(id) => {
              setSelectedId(id);
              setCreating(false);
            }}
            onCancel={() => setCreating(false)}
          />
        </Card>
      )}

      {isLoading ? (
        <Spinner />
      ) : isError ? (
        <ErrorBox error={error} />
      ) : !data || data.length === 0 ? (
        <Card><Empty>Пока нет планов. Создайте первый.</Empty></Card>
      ) : (
        <>
          <div className="flex flex-wrap gap-2">
            {data.map((p) => (
              <button
                key={p.id}
                onClick={() => setSelectedId(p.id)}
                className={`rounded-lg border px-3 py-1.5 text-sm ${
                  selected?.id === p.id
                    ? "border-brand-500 bg-brand-50 text-brand-700"
                    : "border-slate-300 text-slate-600 hover:bg-slate-50"
                }`}
              >
                {p.name || "Без названия"}
                <span className="ml-2 text-xs text-slate-400">
                  {shortDate(p.start_date)} – {shortDate(p.end_date)}
                </span>
              </button>
            ))}
          </div>

          {selected && <PlanGrid planId={selected.id} />}
        </>
      )}
    </div>
  );
}
