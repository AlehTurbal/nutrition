import { useEffect, useRef, useState } from "react";
import {
  useChatMessages,
  useChatThreads,
  useCreateThread,
  useDeleteThread,
  useSendMessage,
} from "../../api/chat";
import { useCreateProduct } from "../../api/products";
import { useCreateRecipe } from "../../api/recipes";
import type {
  ChatProposal,
  ProductInput,
  RecipeInput,
} from "../../api/types";
import { Button } from "../../components/ui";

interface PendingProposal {
  key: string;
  proposal: ChatProposal;
  applied: boolean;
  error?: string;
}

export default function ChatPanel() {
  const threads = useChatThreads();
  const createThread = useCreateThread();
  const deleteThread = useDeleteThread();
  const [activeId, setActiveId] = useState<number | null>(null);

  // Default to the newest thread once threads load.
  useEffect(() => {
    if (activeId == null && threads.data && threads.data.length > 0) {
      setActiveId(threads.data[0].id);
    }
  }, [threads.data, activeId]);

  const messages = useChatMessages(activeId);
  const send = useSendMessage(activeId ?? 0);
  const createProduct = useCreateProduct();
  const createRecipe = useCreateRecipe();

  const [input, setInput] = useState("");
  const [proposals, setProposals] = useState<PendingProposal[]>([]);
  const feedRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    feedRef.current?.scrollTo({ top: feedRef.current.scrollHeight });
  }, [messages.data, proposals, send.isPending]);

  async function startThread() {
    const t = await createThread.mutateAsync("Новый чат");
    setActiveId(t.id);
    setProposals([]);
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
        await createProduct.mutateAsync(p.proposal.payload as ProductInput);
      } else {
        await createRecipe.mutateAsync(p.proposal.payload as RecipeInput);
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

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      {/* Thread selector */}
      <div className="flex items-center gap-1 border-b border-slate-100 px-3 py-2">
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
        <button
          onClick={startThread}
          title="Новый чат"
          className="rounded-md px-2 py-1 text-slate-500 hover:bg-slate-100"
        >
          ＋
        </button>
        {activeId != null && (
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
            <div
              key={m.id}
              className={
                m.role === "user"
                  ? "ml-6 rounded-lg bg-brand-50 px-3 py-2 text-sm text-slate-800"
                  : "mr-6 rounded-lg bg-slate-100 px-3 py-2 text-sm text-slate-700"
              }
            >
              {m.content}
            </div>
          ))
        )}

        {send.isPending && (
          <div className="mr-6 rounded-lg bg-slate-100 px-3 py-2 text-sm text-slate-400">
            …
          </div>
        )}

        {proposals.map((p) => (
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
  const name = (proposal.payload as { name?: string }).name ?? "—";
  const label = proposal.type === "product" ? "Продукт" : "Рецепт";

  return (
    <div className="mr-6 rounded-lg border border-brand-200 bg-white px-3 py-2 text-sm">
      <div className="mb-1 text-xs font-medium text-brand-700">
        Предложение · {label}
      </div>
      <div className="font-medium text-slate-800">{name}</div>
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
