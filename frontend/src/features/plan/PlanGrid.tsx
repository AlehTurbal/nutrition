import { useEffect, useRef, useState } from "react";
import {
  useCopyDay,
  useDeleteItem,
  useDeletePlan,
  usePlan,
  useShoppingList,
  useUpdateItem,
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
  const [copySource, setCopySource] = useState<string | null>(null);
  const [editingItem, setEditingItem] = useState<number | null>(null);

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
  const plannedByCell = new Map(
    (shopping.data?.by_cell ?? []).map((c) => [`${c.date}|${c.slot}`, c.macros]),
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
                <td className="border-b border-slate-100 p-2 align-top whitespace-nowrap">
                  <div className="font-medium text-slate-600">
                    {shortDate(day)}
                  </div>
                  {copySource === day ? (
                    <CopyDayPanel
                      planId={planId}
                      source={day}
                      days={days}
                      onDone={() => setCopySource(null)}
                    />
                  ) : (
                    <button
                      onClick={() => setCopySource(day)}
                      title="Копировать день"
                      className="mt-1 text-xs text-slate-400 hover:text-brand-500"
                    >
                      ⧉ копировать
                    </button>
                  )}
                </td>
                {slots.map((slot) => {
                  const cellKey = `${day}|${slot}`;
                  const cellItems = itemsAt(day, slot);
                  const cellMacros = plannedByCell.get(cellKey);
                  return (
                    <td
                      key={slot}
                      className="border-b border-l border-slate-100 p-2 align-top"
                    >
                      <div className="space-y-1">
                        {cellItems.length > 0 && cellMacros && (
                          <div className="text-[11px] text-slate-400">
                            {cellMacros.complete ? "" : "≈"}Б:{fmt(cellMacros.protein)} Ж:
                            {fmt(cellMacros.fat)} У:{fmt(cellMacros.carbs)}
                          </div>
                        )}
                        {cellItems.map((it) => (
                          <div key={it.id} className="relative">
                            <div className="group flex items-center justify-between gap-1 rounded bg-brand-50 px-2 py-1 text-xs text-brand-700">
                              <button
                                type="button"
                                onClick={() =>
                                  setEditingItem(
                                    editingItem === it.id ? null : it.id,
                                  )
                                }
                                title="Изменить количество"
                                className="flex-1 text-left hover:underline"
                              >
                                {it.product_id != null
                                  ? `${it.product_name} ${fmt(it.grams ?? 0)} г`
                                  : `${it.recipe_name}${it.servings !== 1 ? ` ×${it.servings}` : ""}`}
                              </button>
                              <button
                                onClick={() => delItem.mutate(it.id)}
                                className="text-brand-400 opacity-0 group-hover:opacity-100"
                              >
                                ✕
                              </button>
                            </div>
                            {editingItem === it.id && (
                              <EditQtyPopover
                                planId={planId}
                                item={it}
                                onDone={() => setEditingItem(null)}
                              />
                            )}
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

// EditQtyPopover is a small floating editor anchored to a plan item. It edits
// grams for a product item or servings (порции) for a recipe item, then saves
// via PATCH. Clicking outside or ✕ closes without saving.
function EditQtyPopover({
  planId,
  item,
  onDone,
}: {
  planId: number;
  item: PlanItem;
  onDone: () => void;
}) {
  const update = useUpdateItem(planId);
  const isProduct = item.product_id != null;
  const [value, setValue] = useState<number>(
    isProduct ? (item.grams ?? 0) : item.servings,
  );
  const ref = useRef<HTMLDivElement>(null);

  // Close on outside click.
  useEffect(() => {
    const onClick = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) onDone();
    };
    document.addEventListener("mousedown", onClick);
    return () => document.removeEventListener("mousedown", onClick);
  }, [onDone]);

  const save = () => {
    if (value <= 0) return;
    update.mutate(
      isProduct ? { itemId: item.id, grams: value } : { itemId: item.id, servings: value },
      { onSuccess: onDone },
    );
  };

  return (
    <div
      ref={ref}
      className="absolute left-0 top-full z-10 mt-1 w-40 space-y-1 rounded border border-slate-200 bg-white p-2 shadow-lg"
    >
      <div className="text-xs text-slate-500">
        {isProduct ? item.product_name : item.recipe_name}
      </div>
      <div className="flex items-center gap-1">
        <input
          type="number"
          autoFocus
          min={isProduct ? 1 : 0.5}
          step={isProduct ? 10 : 0.5}
          value={value}
          onChange={(e) => setValue(Number(e.target.value))}
          onKeyDown={(e) => e.key === "Enter" && save()}
          className="w-16 rounded border border-slate-300 px-1 py-1 text-xs"
        />
        <span className="text-xs text-slate-400">{isProduct ? "г" : "порц."}</span>
      </div>
      <div className="flex items-center gap-1">
        <Button
          type="button"
          onClick={save}
          disabled={update.isPending || value <= 0}
          className="px-2 py-0.5 text-xs"
        >
          Сохранить
        </Button>
        <Button
          type="button"
          variant="ghost"
          onClick={onDone}
          className="px-2 py-0.5 text-xs"
        >
          ✕
        </Button>
      </div>
      {update.isError && (
        <div className="text-xs text-red-600">
          {(update.error as Error).message}
        </div>
      )}
    </div>
  );
}

// CopyDayPanel copies one day's items onto a chosen day or the whole plan
// ("Вся неделя"). Targets are replaced with a copy of the source day.
function CopyDayPanel({
  planId,
  source,
  days,
  onDone,
}: {
  planId: number;
  source: string;
  days: string[];
  onDone: () => void;
}) {
  const copy = useCopyDay(planId);
  const others = days.filter((d) => d !== source);
  const [target, setTarget] = useState<string>("all");

  if (others.length === 0) {
    return (
      <div className="mt-1 text-xs text-slate-400">
        нет других дней{" "}
        <button onClick={onDone} className="text-brand-500">
          ✕
        </button>
      </div>
    );
  }

  const submit = () => {
    const target_dates = target === "all" ? others : [target];
    copy.mutate({ source_date: source, target_dates }, { onSuccess: onDone });
  };

  return (
    <div className="mt-1 space-y-1 rounded border border-slate-200 p-1.5">
      <select
        value={target}
        onChange={(e) => setTarget(e.target.value)}
        className="w-full rounded border border-slate-300 px-1 py-1 text-xs"
      >
        <option value="all">Вся неделя</option>
        {others.map((d) => (
          <option key={d} value={d}>
            {shortDate(d)}
          </option>
        ))}
      </select>
      <div className="flex items-center gap-1">
        <Button
          type="button"
          onClick={submit}
          disabled={copy.isPending}
          className="px-2 py-0.5 text-xs"
        >
          ОК
        </Button>
        <Button
          type="button"
          variant="ghost"
          onClick={onDone}
          className="px-2 py-0.5 text-xs"
        >
          ✕
        </Button>
      </div>
      {copy.isError && (
        <div className="text-xs text-red-600">
          {(copy.error as Error).message}
        </div>
      )}
    </div>
  );
}
