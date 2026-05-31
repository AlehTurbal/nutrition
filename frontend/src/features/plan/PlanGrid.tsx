import { useState } from "react";
import { useDeleteItem, useDeletePlan, usePlan } from "../../api/plans";
import { useProfile } from "../../api/profile";
import { useRecipes } from "../../api/recipes";
import type { PlanItem } from "../../api/types";
import { Button, Card, ErrorBox, Spinner } from "../../components/ui";
import { dateRange, shortDate } from "../../lib/dates";
import { slotLabel } from "../../lib/labels";
import AddItem from "./AddItem";

const DEFAULT_SLOTS = ["breakfast", "lunch", "dinner"];

export default function PlanGrid({ planId }: { planId: number }) {
  const plan = usePlan(planId);
  const profile = useProfile();
  const recipes = useRecipes();
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
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {delItem.isError && <div className="mt-3"><ErrorBox error={delItem.error} /></div>}
    </Card>
  );
}
