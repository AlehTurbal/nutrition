import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "./client";
import type { ChatMessage, ChatSendResponse, ChatThread } from "./types";

export function useChatThreads() {
  return useQuery({
    queryKey: ["chat", "threads"],
    queryFn: () => api.get<ChatThread[]>("/api/chat/threads"),
  });
}

export function useCreateThread() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (title: string) =>
      api.post<ChatThread>("/api/chat/threads", { title }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["chat", "threads"] }),
  });
}

export function useDeleteThread() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => api.del<void>(`/api/chat/threads/${id}`),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["chat", "threads"] }),
  });
}

export function useChatMessages(threadId: number | null) {
  return useQuery({
    queryKey: ["chat", "messages", threadId],
    queryFn: () =>
      api.get<ChatMessage[]>(`/api/chat/threads/${threadId}/messages`),
    enabled: threadId != null,
  });
}

export function useSendMessage(threadId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (content: string) =>
      api.post<ChatSendResponse>(
        `/api/chat/threads/${threadId}/messages`,
        { content },
      ),
    onSuccess: () =>
      qc.invalidateQueries({ queryKey: ["chat", "messages", threadId] }),
  });
}
