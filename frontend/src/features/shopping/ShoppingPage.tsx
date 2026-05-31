import { useEffect, useState } from "react";
import { usePlans, useShoppingList } from "../../api/plans";
import { Card, Empty, ErrorBox, Spinner } from "../../components/ui";
import { fmt, slotLabel } from "../../lib/labels";
import { shortDate } from "../../lib/dates";

export default function ShoppingPage() {
  const plans = usePlans();
  const [planId, setPlanId] = useState<number | null>(null);

  useEffect(() => {
    if (planId == null && plans.data && plans.data.length > 0) {
      setPlanId(plans.data[0].id);
    }
  }, [plans.data, planId]);

  const list = useShoppingList(planId);

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold text-slate-800">Список покупок</h1>

      {plans.isLoading ? (
        <Spinner />
      ) : plans.isError ? (
        <ErrorBox error={plans.error} />
      ) : !plans.data || plans.data.length === 0 ? (
        <Card><Empty>Сначала создайте план питания</Empty></Card>
      ) : (
        <>
          <select
            value={planId ?? ""}
            onChange={(e) => setPlanId(Number(e.target.value))}
            className="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-sm"
          >
            {plans.data.map((p) => (
              <option key={p.id} value={p.id}>
                {p.name || "Без названия"} ({shortDate(p.start_date)}–
                {shortDate(p.end_date)})
              </option>
            ))}
          </select>

          {list.isLoading ? (
            <Spinner />
          ) : list.isError ? (
            <ErrorBox error={list.error} />
          ) : !list.data || list.data.items.length === 0 ? (
            <Card><Empty>В этом плане ещё нет блюд</Empty></Card>
          ) : (
            <div className="grid gap-6 lg:grid-cols-3">
              <div className="lg:col-span-2">
                <Card title="Нужно купить">
                  <table className="w-full text-sm">
                    <thead>
                      <tr className="text-left text-xs uppercase text-slate-400">
                        <th className="py-2">Продукт</th>
                        <th className="py-2 text-right">Граммы</th>
                        <th className="py-2 text-right">Ккал</th>
                      </tr>
                    </thead>
                    <tbody>
                      {list.data.items.map((it) => (
                        <tr key={it.product_id} className="border-t border-slate-100">
                          <td className="py-2 font-medium text-slate-700">
                            {it.product_name}
                            {!it.macros.complete && (
                              <span className="ml-1 text-amber-500" title="Неполные данные">⚠</span>
                            )}
                          </td>
                          <td className="py-2 text-right">{fmt(it.grams)} г</td>
                          <td className="py-2 text-right">{fmt(it.macros.kcal)}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </Card>
              </div>

              <div className="space-y-6">
                <Card title="Итого БЖУ">
                  <Totals m={list.data.totals} />
                  {!list.data.totals.complete && (
                    <p className="mt-2 text-xs text-amber-600">
                      ⚠ У части продуктов не заполнен БЖУ — суммы занижены.
                    </p>
                  )}
                </Card>

                <Card title="По приёмам">
                  <div className="space-y-2 text-sm">
                    {list.data.by_slot.map((s) => (
                      <div key={s.slot} className="flex justify-between">
                        <span className="text-slate-600">{slotLabel(s.slot)}</span>
                        <span className="font-medium text-slate-700">
                          {fmt(s.macros.kcal)} ккал
                        </span>
                      </div>
                    ))}
                  </div>
                </Card>
              </div>
            </div>
          )}
        </>
      )}
    </div>
  );
}

function Totals({ m }: { m: { kcal: number; protein: number; fat: number; carbs: number } }) {
  const rows = [
    ["Калории", m.kcal, "ккал"],
    ["Белки", m.protein, "г"],
    ["Жиры", m.fat, "г"],
    ["Углеводы", m.carbs, "г"],
  ] as const;
  return (
    <div className="space-y-1 text-sm">
      {rows.map(([label, val, unit]) => (
        <div key={label} className="flex justify-between">
          <span className="text-slate-600">{label}</span>
          <span className="font-medium text-slate-700">
            {fmt(val)} {unit}
          </span>
        </div>
      ))}
    </div>
  );
}
