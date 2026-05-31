import { useSyncExternalStore } from "react";
import { Navigate, Route, Routes } from "react-router-dom";
import { getToken, subscribe } from "./lib/auth";
import Layout from "./components/Layout";
import AuthPage from "./features/auth/AuthPage";
import DashboardPage from "./features/dashboard/DashboardPage";
import ProductsPage from "./features/products/ProductsPage";
import RecipesPage from "./features/recipes/RecipesPage";
import PlanPage from "./features/plan/PlanPage";
import ShoppingPage from "./features/shopping/ShoppingPage";
import StorePage from "./features/store/StorePage";

function useToken() {
  return useSyncExternalStore(subscribe, getToken);
}

export default function App() {
  const token = useToken();

  if (!token) return <AuthPage />;

  return (
    <Layout>
      <Routes>
        <Route path="/" element={<DashboardPage />} />
        <Route path="/products" element={<ProductsPage />} />
        <Route path="/recipes" element={<RecipesPage />} />
        <Route path="/plan" element={<PlanPage />} />
        <Route path="/shopping" element={<ShoppingPage />} />
        <Route path="/store" element={<StorePage />} />
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </Layout>
  );
}
