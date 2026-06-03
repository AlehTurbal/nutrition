import { useState } from "react";
import {
  useDeleteItem,
  useDeletePlan,
  usePlan,
  useShoppingList,
} from "../../api/plans";
import { useProfile, useTargets } from "../../api/profile";
import { useRecipes } from "../../api/recipes";
import type { Macros, PlanItem } from "../../api/types";
import { Button, Card, ErrorBox, Spinner } from "../../components/ui";
import { dateRange, shortDate } from "../../lib/dates";
import { fmt, slotLabel } from "../../lib/labels";
import AddItem from "./AddItem";

const ZERO_MACROS: Macros = {
  kcal: 0,
  protein: 0,
  fat: 0,
  carbs: 0,
  complete: true,
};

// RemainingCell shows the БЖУ left for a day (daily target − planned), or just
// the planned totals when no daily target is available (no profile/weight yet).
function RemainingCell({
  planned,
  target,
}: {
  planned: Macros;
  target: { calories: number; protein_g: number; fat_g: number; carbs_g: number } | null;
}) {
  const approx = planned.complete ? "" : "≈";
  // [label, value, signed (=is a remainder), norm]. When a daily target exists
  // each row shows the remaining amount plus the target norm in parentheses,
  // e.g. "120 (170)".
  const rows: [string, number, boolean, number | null][] = target
    ? [
        ["ккал", target.calories - planned.kcal, true, target.calories],
        ["Б", target.protein_g - planned.protein, true, target.protein_g],
        ["Ж", target.fat_g - planned.fat, true, target.fat_g],
        ["У", target.carbs_g - planned.carbs, true, target.carbs_g],
      ]
    : [
        ["ккал", planned.kcal, false, null],
        ["Б", planned.protein, false, null],
        ["Ж", planned.fat, false, null],
        ["У", planned.carbs, false, null],
      ];
  return (
    <div className="space-y-0.5 text-xs">
      {rows.map(([label, value, signed, norm]) => (
        <div key={label} className="flex justify-between gap-2">
          <span className="text-slate-400">{label}</span>
          <span
            className={
              signed && value < 0 ? "text-red-500" : "text-slate-600"
            }
          >
            {approx}
            {fmt(value)}
            {norm != null && (
              <span className="text-slate-400"> ({fmt(norm)})</span>
            )}
          </span>
        </div>
      ))}
    </div>
  );
}

const DEFAULT_SLOTS = ["breakfast", "lunch", "dinner"];

export default function PlanGrid({ planId }: { planId: number }) {
  const plan = usePlan(planId);
  const profile = useProfile();
  const recipes = useRecipes();
  const shopping = useShoppingList(planId);
  const targets = useTargets();
  const delItem = useDeleteItem(planId);
  const delPlan = useDeletePlan();
  const [openCell, setOpenCell] = useState<string | null>(null);

  if (plan.isLoading) return <Spinner />;
  if (plan.isError) return <ErrorBox error={plan.error} />;
  if (!plan.data) return null;

  const slots = profile.data?.meal_slots?.length
    ? profile.data.meal_slots
    : DEFAULT_SLOTS;
  const days = dateRange(plan.data.start_date, plan.data.end_date);

  const itemsAt = (day: string, slot: string): PlanItem[] =>
    plan.data!.items.filter((i) => i.day_date === day && i.meal_slot === slot);

  const plannedByDay = new Map(
    (shopping.data?.by_day ?? []).map((d) => [d.date, d.macros]),
  );
  const dayTarget = targets.data?.targets ?? null;

  return (
    <Card
      title={plan.data.name || "План"}
      actions={
        <Button
          variant="danger"
          onClick={() =>
            confirm("Удалить план целиком?") && delPlan.mutate(planId)
          }
        >
          Удалить план
        </Button>
      }
    >
      <div className="overflow-x-auto">
        <table className="w-full border-collapse text-sm">
          <thead>
            <tr>
              <th className="border-b border-slate-200 p-2 text-left text-xs uppercase text-slate-400">
                День
              </th>
              {slots.map((s) => (
                <th
                  key={s}
                  className="border-b border-slate-200 p-2 text-left text-xs uppercase text-slate-400"
                >
                  {slotLabel(s)}
                </th>
              ))}
              <th className="border-b border-l border-slate-200 p-2 text-left text-xs uppercase text-slate-400">
                {dayTarget ? "Остаток" : "Итог за день"}
              </th>
            </tr>
          </thead>
          <tbody>
            {days.map((day) => (
              <tr key={day} className="align-top">
                <td className="border-b border-slate-100 p-2 font-medium text-slate-600 whitespace-nowrap">
                  {shortDate(day)}
                </td>
                {slots.map((slot) => {
                  const cellKey = `${day}|${slot}`;
                  return (
                    <td
                      key={slot}
                      className="border-b border-l border-slate-100 p-2 align-top"
                    >
                      <div className="space-y-1">
                        {itemsAt(day, slot).map((it) => (
                          <div
                            key={it.id}
                            className="group flex items-center justify-between gap-1 rounded bg-brand-50 px-2 py-1 text-xs text-brand-700"
                          >
                            <span>
                              {it.recipe_name}
                              {it.servings !== 1 && ` ×${it.servings}`}
                            </span>
                            <button
                              onClick={() => delItem.mutate(it.id)}
                              className="text-brand-400 opacity-0 group-hover:opacity-100"
                            >
                              ✕
                            </button>
                          </div>
                        ))}

                        {openCell === cellKey ? (
                          <AddItem
                            planId={planId}
                            day={day}
                            slot={slot}
                            recipes={recipes.data ?? []}
                            onDone={() => setOpenCell(null)}
                          />
                        ) : (
                          <button
                            onClick={() => setOpenCell(cellKey)}
                            className="w-full rounded border border-dashed border-slate-300 py-1 text-xs text-slate-400 hover:border-brand-400 hover:text-brand-500"
                          >
                            +
                          </button>
                        )}
                      </div>
                    </td>
                  );
                })}
                <td className="border-b border-l border-slate-100 p-2 align-top">
                  <RemainingCell
                    planned={plannedByDay.get(day) ?? ZERO_MACROS}
                    target={dayTarget}
                  />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {delItem.isError && <div className="mt-3"><ErrorBox error={delItem.error} /></div>}
    </Card>
  );
}
