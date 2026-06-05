import { Fragment, useEffect, useRef, useState } from "react";
import {
  useChatMessages,
  useChatThreads,
  useCreateThread,
  useDeleteThread,
  useSendMessage,
  useUpdateThread,
} from "../../api/chat";
import { useApplyAddToPlan, useApplyCopyDay } from "../../api/plans";
import { useCreateProduct, useUpdateProduct } from "../../api/products";
import { useCreateRecipe } from "../../api/recipes";
import type { ChatProposal } from "../../api/types";
import { Button } from "../../components/ui";

interface PendingProposal {
  key: string;
  messageId: number;
  proposal: ChatProposal;
  applied: boolean;
  error?: string;
}

export default function ChatPanel() {
  const threads = useChatThreads();
  const createThread = useCreateThread();
  const deleteThread = useDeleteThread();
  const updateThread = useUpdateThread();
  const [activeId, setActiveId] = useState<number | null>(null);
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState("");

  // Default to the newest thread once threads load.
  useEffect(() => {
    if (activeId == null && threads.data && threads.data.length > 0) {
      setActiveId(threads.data[0].id);
    }
  }, [threads.data, activeId]);

  const messages = useChatMessages(activeId);
  const send = useSendMessage(activeId ?? 0);
  const createProduct = useCreateProduct();
  const updateProduct = useUpdateProduct();
  const createRecipe = useCreateRecipe();
  const copyDay = useApplyCopyDay();
  const addToPlan = useApplyAddToPlan();

  const [input, setInput] = useState("");
  const [proposals, setProposals] = useState<PendingProposal[]>([]);
  const feedRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    feedRef.current?.scrollTo({ top: feedRef.current.scrollHeight });
  }, [messages.data, proposals, send.isPending]);

  // Proposals are ephemeral client state tied to one thread; drop them when the
  // active thread changes so they don't leak into another conversation.
  useEffect(() => {
    setProposals([]);
    setEditing(false);
  }, [activeId]);

  // Leave the title empty so it gets named after the user's first request
  // (the dropdown shows "Чат #N" until then).
  async function startThread() {
    const t = await createThread.mutateAsync("");
    setActiveId(t.id);
    setProposals([]);
  }

  const activeThread = threads.data?.find((t) => t.id === activeId);

  function startRename() {
    if (activeThread == null) return;
    setDraft(activeThread.title);
    setEditing(true);
  }

  async function commitRename() {
    const title = draft.trim();
    if (activeId == null || title === "") {
      setEditing(false);
      return;
    }
    await updateThread.mutateAsync({ id: activeId, title });
    setEditing(false);
  }

  async function removeThread(id: number) {
    await deleteThread.mutateAsync(id);
    if (activeId === id) {
      setActiveId(null);
      setProposals([]);
    }
  }

  async function handleSend() {
    const content = input.trim();
    if (!content || activeId == null) return;
    setInput("");
    try {
      const res = await send.mutateAsync(content);
      if (res.proposals.length) {
        setProposals((prev) => [
          ...prev,
          ...res.proposals.map((proposal, i) => ({
            key: `${res.message.id}-${i}`,
            messageId: res.message.id,
            proposal,
            applied: false,
          })),
        ]);
      }
    } catch {
      // Surfaced via send.error below; restore the text so it isn't lost.
      setInput(content);
    }
  }

  async function applyProposal(p: PendingProposal) {
    try {
      if (p.proposal.type === "product") {
        // A product_id means "update this existing product" — PUT it instead of
        // POSTing a duplicate. Strip the id from the body: the update endpoint
        // takes it from the URL and rejects unknown fields.
        const { product_id, ...input } = p.proposal.payload;
        if (product_id != null) {
          await updateProduct.mutateAsync({ id: product_id, input });
        } else {
          await createProduct.mutateAsync(input);
        }
      } else if (p.proposal.type === "recipe") {
        await createRecipe.mutateAsync(p.proposal.payload);
      } else if (p.proposal.type === "copy_day") {
        await copyDay.mutateAsync(p.proposal.payload);
      } else {
        await addToPlan.mutateAsync(p.proposal.payload);
      }
      setProposals((ps) =>
        ps.map((x) => (x.key === p.key ? { ...x, applied: true } : x)),
      );
    } catch (e) {
      const msg = e instanceof Error ? e.message : "ошибка";
      setProposals((ps) =>
        ps.map((x) => (x.key === p.key ? { ...x, error: msg } : x)),
      );
    }
  }

  function rejectProposal(key: string) {
    setProposals((ps) => ps.filter((x) => x.key !== key));
  }

  // Proposals render inline under their assistant message. A just-created
  // proposal whose message hasn't been refetched yet is shown at the bottom as a
  // fallback so it never flickers out of view.
  const loadedMessageIds = new Set(messages.data?.map((m) => m.id) ?? []);
  const orphanProposals = proposals.filter(
    (p) => !loadedMessageIds.has(p.messageId),
  );

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      {/* Thread selector */}
      <div className="flex items-center gap-1 border-b border-slate-100 px-3 py-2">
        {editing ? (
          <input
            autoFocus
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                commitRename();
              } else if (e.key === "Escape") {
                e.preventDefault();
                setEditing(false);
              }
            }}
            disabled={updateThread.isPending}
            className="min-w-0 flex-1 rounded-md border border-slate-300 bg-white px-2 py-1 text-xs outline-none focus:border-brand-500 disabled:bg-slate-50"
          />
        ) : (
          <select
            value={activeId ?? ""}
            onChange={(e) =>
              setActiveId(e.target.value ? Number(e.target.value) : null)
            }
            className="min-w-0 flex-1 rounded-md border border-slate-300 bg-white px-2 py-1 text-xs outline-none focus:border-brand-500"
          >
            {(!threads.data || threads.data.length === 0) && (
              <option value="">Нет чатов</option>
            )}
            {threads.data?.map((t) => (
              <option key={t.id} value={t.id}>
                {t.title || `Чат #${t.id}`}
              </option>
            ))}
          </select>
        )}
        {editing ? (
          <>
            <button
              onClick={commitRename}
              disabled={updateThread.isPending || draft.trim() === ""}
              title="Сохранить"
              className="rounded-md px-2 py-1 text-green-600 hover:bg-green-50 disabled:text-slate-300 disabled:hover:bg-transparent"
            >
              ✓
            </button>
            <button
              onClick={() => setEditing(false)}
              disabled={updateThread.isPending}
              title="Отмена"
              className="rounded-md px-2 py-1 text-slate-400 hover:bg-slate-100"
            >
              ✕
            </button>
          </>
        ) : (
          <>
            {activeId != null && (
              <button
                onClick={startRename}
                title="Переименовать чат"
                className="rounded-md px-2 py-1 text-slate-500 hover:bg-slate-100"
              >
                ✎
              </button>
            )}
            <button
              onClick={startThread}
              title="Новый чат"
              className="rounded-md px-2 py-1 text-slate-500 hover:bg-slate-100"
            >
              ＋
            </button>
          </>
        )}
        {activeId != null && !editing && (
          <button
            onClick={() => removeThread(activeId)}
            title="Удалить чат"
            className="rounded-md px-2 py-1 text-slate-400 hover:bg-red-50 hover:text-red-600"
          >
            🗑
          </button>
        )}
      </div>

      {/* Message feed */}
      <div ref={feedRef} className="min-h-0 flex-1 space-y-2 overflow-y-auto p-3">
        {activeId == null ? (
          <p className="mt-6 text-center text-xs text-slate-400">
            Создайте чат, чтобы начать.
          </p>
        ) : messages.data && messages.data.length === 0 && !send.isPending ? (
          <p className="mt-6 text-center text-xs text-slate-400">
            Спросите что-нибудь — например «добавь продукт творог 5%».
          </p>
        ) : (
          messages.data?.map((m) => (
            <Fragment key={m.id}>
              <div
                className={
                  m.role === "user"
                    ? "ml-6 rounded-lg bg-brand-50 px-3 py-2 text-sm text-slate-800"
                    : "mr-6 rounded-lg bg-slate-100 px-3 py-2 text-sm text-slate-700"
                }
              >
                {m.content}
              </div>
              {proposals
                .filter((p) => p.messageId === m.id)
                .map((p) => (
                  <ProposalCard
                    key={p.key}
                    pending={p}
                    onApply={() => applyProposal(p)}
                    onReject={() => rejectProposal(p.key)}
                  />
                ))}
            </Fragment>
          ))
        )}

        {send.isPending && (
          <div className="mr-6 rounded-lg bg-slate-100 px-3 py-2 text-sm text-slate-400">
            …
          </div>
        )}

        {orphanProposals.map((p) => (
          <ProposalCard
            key={p.key}
            pending={p}
            onApply={() => applyProposal(p)}
            onReject={() => rejectProposal(p.key)}
          />
        ))}
      </div>

      {/* Errors + input */}
      {send.error && (
        <p className="px-3 pb-1 text-xs text-red-600">
          {(send.error as Error).message}
        </p>
      )}
      <div className="flex items-end gap-1 border-t border-slate-100 p-2">
        <textarea
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && !e.shiftKey) {
              e.preventDefault();
              handleSend();
            }
          }}
          rows={2}
          placeholder={activeId == null ? "Сначала создайте чат" : "Сообщение…"}
          disabled={activeId == null || send.isPending}
          className="min-w-0 flex-1 resize-none rounded-lg border border-slate-300 px-2 py-1 text-sm outline-none focus:border-brand-500 disabled:bg-slate-50"
        />
        <Button
          onClick={handleSend}
          disabled={activeId == null || send.isPending || !input.trim()}
        >
          ➤
        </Button>
      </div>
    </div>
  );
}

function ProposalCard({
  pending,
  onApply,
  onReject,
}: {
  pending: PendingProposal;
  onApply: () => void;
  onReject: () => void;
}) {
  const { proposal, applied, error } = pending;
  const label =
    proposal.type === "product"
      ? proposal.payload.product_id != null
        ? "Продукт · обновление"
        : "Продукт"
      : proposal.type === "recipe"
        ? "Рецепт"
        : proposal.type === "copy_day"
          ? "Копирование дня"
          : "Добавить в план";
  const title =
    proposal.type === "copy_day"
      ? `${proposal.payload.source_date} → ${proposal.payload.target_dates.length} дн.`
      : proposal.type === "add_to_plan"
        ? `${proposal.payload.day_date} · ${proposal.payload.meal_slot} · ${proposal.payload.grams} г`
        : proposal.payload.name || "—";

  return (
    <div className="mr-6 rounded-lg border border-brand-200 bg-white px-3 py-2 text-sm">
      <div className="mb-1 text-xs font-medium text-brand-700">
        Предложение · {label}
      </div>
      <div className="font-medium text-slate-800">{title}</div>
      {error && <div className="mt-1 text-xs text-red-600">{error}</div>}
      {applied ? (
        <div className="mt-1 text-xs text-green-600">✓ Применено</div>
      ) : (
        <div className="mt-2 flex gap-2">
          <Button onClick={onApply}>Применить</Button>
          <Button variant="ghost" onClick={onReject}>
            Отклонить
          </Button>
        </div>
      )}
    </div>
  );
}
