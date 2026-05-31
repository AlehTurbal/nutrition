import { useState } from "react";
import {
  useCreateProduct,
  useDeleteProduct,
  useProducts,
  useUpdateProduct,
} from "../../api/products";
import type { Product, ProductInput } from "../../api/types";
import { Button, Card, Empty, ErrorBox, Spinner } from "../../components/ui";
import { fmt } from "../../lib/labels";
import ProductForm from "./ProductForm";

export default function ProductsPage() {
  const { data, isLoading, isError, error } = useProducts();
  const create = useCreateProduct();
  const update = useUpdateProduct();
  const del = useDeleteProduct();
  const [editing, setEditing] = useState<Product | null>(null);
  const [adding, setAdding] = useState(false);

  const submit = (input: ProductInput) => {
    if (editing) {
      update.mutate(
        { id: editing.id, input },
        { onSuccess: () => setEditing(null) },
      );
    } else {
      create.mutate(input, { onSuccess: () => setAdding(false) });
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold text-slate-800">Продукты</h1>
        {!adding && !editing && (
          <Button onClick={() => setAdding(true)}>+ Продукт</Button>
        )}
      </div>

      {(adding || editing) && (
        <Card title={editing ? "Редактирование продукта" : "Новый продукт"}>
          <ProductForm
            initial={editing ?? undefined}
            pending={create.isPending || update.isPending}
            error={create.error ?? update.error}
            onSubmit={submit}
            onCancel={() => {
              setAdding(false);
              setEditing(null);
            }}
          />
        </Card>
      )}

      <Card>
        {isLoading ? (
          <Spinner />
        ) : isError ? (
          <ErrorBox error={error} />
        ) : !data || data.length === 0 ? (
          <Empty>Пока нет продуктов</Empty>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-left text-xs uppercase text-slate-400">
                  <th className="py-2">Название</th>
                  <th className="py-2">Категория</th>
                  <th className="py-2 text-right">Ккал</th>
                  <th className="py-2 text-right">Б</th>
                  <th className="py-2 text-right">Ж</th>
                  <th className="py-2 text-right">У</th>
                  <th className="py-2"></th>
                </tr>
              </thead>
              <tbody>
                {data.map((p) => (
                  <tr key={p.id} className="border-t border-slate-100">
                    <td className="py-2 font-medium text-slate-700">{p.name}</td>
                    <td className="py-2 text-slate-500">{p.category || "—"}</td>
                    <td className="py-2 text-right">{cell(p.kcal100)}</td>
                    <td className="py-2 text-right">{cell(p.protein100)}</td>
                    <td className="py-2 text-right">{cell(p.fat100)}</td>
                    <td className="py-2 text-right">{cell(p.carbs100)}</td>
                    <td className="py-2 text-right">
                      <div className="flex justify-end gap-2">
                        <Button variant="ghost" onClick={() => setEditing(p)}>
                          ✎
                        </Button>
                        <Button
                          variant="danger"
                          onClick={() =>
                            confirm(`Удалить «${p.name}»?`) && del.mutate(p.id)
                          }
                        >
                          ✕
                        </Button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            {del.isError && <div className="mt-3"><ErrorBox error={del.error} /></div>}
          </div>
        )}
      </Card>
    </div>
  );
}

function cell(v: number | null): string {
  return v == null ? "—" : fmt(v);
}
