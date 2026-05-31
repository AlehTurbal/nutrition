import { useEffect, useState } from "react";
import { usePlans } from "../../api/plans";
import { useStoreMatch, useStoreMatches } from "../../api/llm";
import { Button, Card, Empty, ErrorBox, Spinner } from "../../components/ui";
import { shortDate } from "../../lib/dates";
import GenerateRecipe from "./GenerateRecipe";

export default function StorePage() {
  const plans = usePlans();
  const [planId, setPlanId] = useState<number | null>(null);
  const [storeText, setStoreText] = useState("");
  const match = useStoreMatch();
  const saved = useStoreMatches(planId);

  useEffect(() => {
    if (planId == null && plans.data && plans.data.length > 0) {
      setPlanId(plans.data[0].id);
    }
  }, [plans.data, planId]);

  const result = match.data ?? saved.data ?? null;

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold text-slate-800">Магазин</h1>

      {plans.isLoading ? (
        <Spinner />
      ) : !plans.data || plans.data.length === 0 ? (
        <Card><Empty>Сначала создайте план питания</Empty></Card>
      ) : (
        <Card title="Подбор товаров под план">
          <div className="space-y-3">
            <select
              value={planId ?? ""}
              onChange={(e) => setPlanId(Number(e.target.value))}
              className="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-sm"
            >
              {plans.data.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name || "Без названия"} ({shortDate(p.start_date)}–{shortDate(p.end_date)})
                </option>
              ))}
            </select>

            <label className="block text-sm">
              <span className="mb-1 block font-medium text-slate-600">
                Вставьте ассортимент магазина (любой язык)
              </span>
              <textarea
                value={storeText}
                onChange={(e) => setStoreText(e.target.value)}
                rows={6}
                className="w-full rounded-lg border border-slate-300 px-3 py-1.5 text-sm outline-none focus:border-brand-500 focus:ring-1 focus:ring-brand-500"
                placeholder="Скопируйте список товаров со страницы магазина…"
              />
            </label>

            {match.isError && <ErrorBox error={match.error} />}
            <Button
              disabled={planId == null || storeText.trim() === "" || match.isPending}
              onClick={() => planId != null && match.mutate({ plan_id: planId, store_text: storeText })}
            >
              {match.isPending ? "Подбор…" : "Подобрать"}
            </Button>
          </div>

          {result && (
            <div className="mt-4 border-t border-slate-100 pt-4">
              <table className="w-full text-sm">
                <thead>
                  <tr className="text-left text-xs uppercase text-slate-400">
                    <th className="py-1">Нужно</th>
                    <th className="py-1">Найдено в магазине</th>
                  </tr>
                </thead>
                <tbody>
                  {result.items.map((it, i) => (
                    <tr key={i} className="border-t border-slate-50">
                      <td className="py-1.5 font-medium text-slate-700">{it.needed}</td>
                      <td className="py-1.5">
                        {it.found ? (
                          it.matched
                        ) : (
                          <span className="text-amber-600">не найдено</span>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Card>
      )}

      <GenerateRecipe />
    </div>
  );
}
