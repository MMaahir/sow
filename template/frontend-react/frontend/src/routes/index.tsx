import { createFileRoute } from "@tanstack/react-router";

import { useGetHealthz } from "../client.gen";

export const Route = createFileRoute("/")({
  component: Home,
});

function Home() {
  const { data, isError } = useGetHealthz();
  let status = "checking";
  if (isError) {
    status = "unreachable";
  } else if (data) {
    status = data.data.status;
  }

  return (
    <main>
      <h1>The frontend is running</h1>
      <p>Backend status: {status}</p>
    </main>
  );
}
