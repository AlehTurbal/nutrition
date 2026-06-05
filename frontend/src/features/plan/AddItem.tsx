import { useState } from "react";
import { useAddItem } from "../../api/plans";
import { useProducts } from "../../api/products";
import type { Recipe } from "../../api/types";
import { Button } from "../../components/ui";

type Mode = "recipe" | "product";

export default function AddItem({
  planId,
  day,
  slot,
  recipes,
  onDone,
}: {
  planId: number;
  day: string;
  slot: string;
  recipes: Recipe[];
  onDone: () => void;
}) {
  const add = useAddItem(planId);
  const products = useProducts();
  const [mode, setMode] = useState<Mode>("recipe");
  const [recipeId, setRecipeId] = useState<number>(recipes[0]?.id ?? 0);
  const [servings, setServings] = useState(1);
  const [productId, setProductId] = useState<number>(0);
  const [grams, setGrams] = useState(100);

  const productList = products.data ?? [];
  // Keep the product select pointed at a real product once the list loads.
  if (mode === "product" && productId === 0 && productList.length > 0) {
    setProductId(productList[0].id);
  }

  const submit = () => {
    if (mode === "recipe") {
      add.mutate(
        { day_date: day, meal_slot: slot, recipe_id: recipeId, servings },
        { onSuccess: onDone },
      );
    } else {
      add.mutate(
        { day_date: day, meal_slot: slot, product_id: productId, grams },
        { onSuccess: onDone },
      );
    }
  };

  const tabClass = (m: Mode) =>
    `flex-1 rounded px-1 py-0.5 text-xs ${
      mode === m ? "bg-brand-500 text-white" : "bg-slate-100 text-slate-500"
    }`;

  return (
    <div className="space-y-1 rounded border border-slate-200 p-2">
      <div className="flex gap-1">
        <button type="button" className={tabClass("recipe")} onClick={() => setMode("recipe")}>
          Рецепт
        </button>
        <button type="button" className={tabClass("product")} onClick={() => setMode("product")}>
          Продукт
        </button>
      </div>

      {mode === "recipe" ? (
        recipes.length === 0 ? (
          <div className="text-xs text-slate-400">Нет рецептов.</div>
        ) : (
          <>
            <select
              value={recipeId}
              onChange={(e) => setRecipeId(Number(e.target.value))}
              className="w-full rounded border border-slate-300 px-1 py-1 text-xs"
            >
              {recipes.map((r) => (
                <option key={r.id} value={r.id}>{r.name}</option>
              ))}
            </select>
            <div className="flex items-center gap-1">
              <input
                type="number"
                min={0.5}
                step={0.5}
                value={servings}
                onChange={(e) => setServings(Number(e.target.value))}
                className="w-16 rounded border border-slate-300 px-1 py-1 text-xs"
              />
              <span className="text-xs text-slate-400">порц.</span>
              <Actions onOk={submit} okDisabled={add.isPending} onDone={onDone} />
            </div>
          </>
        )
      ) : productList.length === 0 ? (
        <div className="text-xs text-slate-400">Нет продуктов.</div>
      ) : (
        <>
          <select
            value={productId}
            onChange={(e) => setProductId(Number(e.target.value))}
            className="w-full rounded border border-slate-300 px-1 py-1 text-xs"
          >
            {productList.map((p) => (
              <option key={p.id} value={p.id}>{p.name}</option>
            ))}
          </select>
          <div className="flex items-center gap-1">
            <input
              type="number"
              min={1}
              step={10}
              value={grams}
              onChange={(e) => setGrams(Number(e.target.value))}
              className="w-16 rounded border border-slate-300 px-1 py-1 text-xs"
            />
            <span className="text-xs text-slate-400">г</span>
            <Actions
              onOk={submit}
              okDisabled={add.isPending || productId === 0 || grams <= 0}
              onDone={onDone}
            />
          </div>
        </>
      )}
    </div>
  );
}

function Actions({
  onOk,
  okDisabled,
  onDone,
}: {
  onOk: () => void;
  okDisabled: boolean;
  onDone: () => void;
}) {
  return (
    <>
      <Button
        type="button"
        onClick={onOk}
        disabled={okDisabled}
        className="ml-auto px-2 py-0.5 text-xs"
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
    </>
  );
}
