import ProfileCard from "./ProfileCard";
import WeightCard from "./WeightCard";
import TargetsCard from "./TargetsCard";

export default function DashboardPage() {
  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold text-slate-800">Дашборд</h1>
      <div className="grid gap-6 xl:grid-cols-2">
        <ProfileCard />
        <WeightCard />
      </div>
      <TargetsCard />
    </div>
  );
}
