import { useTargets } from "../../api/profile";
import { Card, ErrorBox, Empty, Spinner } from "../../components/ui";
import { fmt, slotLabel } from "../../lib/labels";

function Stat({ label, value, unit }: { label: string; value: number; unit: string }) {
  return (
    <div className="rounded-lg bg-slate-50 p-3 text-center">
      <div className="text-xl font-semibold text-slate-800">{fmt(value)}</div>
      <div className="text-xs text-slate-500">
        {label} · {unit}
      </div>
    </div>
  );
}

export default function TargetsCard() {
  const { data, isLoading, isError, error } = useTargets();

  return (
    <Card title="Суточная норма">
      {isLoading ? (
        <Spinner />
      ) : isError ? (
        <ErrorBox error={error} />
      ) : !data ? (
        <Empty>Заполните профиль и добавьте вес, чтобы увидеть норму</Empty>
      ) : (
        <div className="space-y-5">
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
            <Stat label="Калории" value={data.targets.calories} unit="ккал" />
            <Stat label="Белки" value={data.targets.protein_g} unit="г" />
            <Stat label="Жиры" value={data.targets.fat_g} unit="г" />
            <Stat label="Углеводы" value={data.targets.carbs_g} unit="г" />
          </div>

          <div>
            <h3 className="mb-2 text-sm font-medium text-slate-600">
              Разбивка по приёмам ({data.per_meal.length})
            </h3>
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="text-left text-xs uppercase text-slate-400">
                    <th className="py-1">Приём</th>
                    <th className="py-1 text-right">Ккал</th>
                    <th className="py-1 text-right">Б</th>
                    <th className="py-1 text-right">Ж</th>
                    <th className="py-1 text-right">У</th>
                  </tr>
                </thead>
                <tbody>
                  {data.per_meal.map((m) => (
                    <tr key={m.slot} className="border-t border-slate-50">
                      <td className="py-1.5 font-medium text-slate-700">
                        {slotLabel(m.slot)}
                      </td>
                      <td className="py-1.5 text-right">{fmt(m.calories)}</td>
                      <td className="py-1.5 text-right">{fmt(m.protein_g)}</td>
                      <td className="py-1.5 text-right">{fmt(m.fat_g)}</td>
                      <td className="py-1.5 text-right">{fmt(m.carbs_g)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}
    </Card>
  );
}
