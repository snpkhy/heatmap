import CompareArrowsIcon from "@mui/icons-material/CompareArrows";
import DeleteIcon from "@mui/icons-material/Delete";
import Alert from "@mui/material/Alert";
import Button from "@mui/material/Button";
import Checkbox from "@mui/material/Checkbox";
import Chip from "@mui/material/Chip";
import IconButton from "@mui/material/IconButton";
import Link from "@mui/material/Link";
import Paper from "@mui/material/Paper";
import Stack from "@mui/material/Stack";
import Table from "@mui/material/Table";
import TableBody from "@mui/material/TableBody";
import TableCell from "@mui/material/TableCell";
import TableContainer from "@mui/material/TableContainer";
import TableHead from "@mui/material/TableHead";
import TableRow from "@mui/material/TableRow";
import Tooltip from "@mui/material/Tooltip";
import Typography from "@mui/material/Typography";
import { useCallback, useEffect, useState } from "react";
import { api, type RunListItem, type RunStatus } from "../api";
import { dateTime, fps, metric, noiseLabel } from "../format";
import { navigate, routeHref } from "../router";

export const statusColor: Record<
  RunStatus,
  "default" | "primary" | "success" | "error" | "warning"
> = {
  queued: "default",
  running: "primary",
  done: "success",
  failed: "error",
  cancelled: "warning",
};

export function RunsPage() {
  const [runs, setRuns] = useState<RunListItem[] | null>(null);
  const [selected, setSelected] = useState<string[]>([]);
  const [error, setError] = useState("");

  const load = useCallback(() => {
    api
      .runs()
      .then((r) => {
        setRuns(r);
        setError("");
      })
      .catch((e: Error) => setError(e.message));
  }, []);

  useEffect(load, [load]);

  // Refresh while something is still in progress.
  const active = runs?.some((r) => r.status === "running" || r.status === "queued");
  useEffect(() => {
    if (!active) return;
    const t = setInterval(load, 1000);
    return () => clearInterval(t);
  }, [active, load]);

  const toggle = (id: string) =>
    setSelected((s) => (s.includes(id) ? s.filter((x) => x !== id) : [...s, id]));

  const remove = async (r: RunListItem) => {
    try {
      await api.deleteRun(r.run_id);
      setSelected((s) => s.filter((x) => x !== r.run_id));
      load();
    } catch (e) {
      setError((e as Error).message);
    }
  };

  return (
    <Stack spacing={3}>
      <Stack direction="row" sx={{ alignItems: "center", justifyContent: "space-between" }}>
        <Typography variant="h5" component="h2">
          Runs
        </Typography>
        <Button
          variant="outlined"
          startIcon={<CompareArrowsIcon />}
          disabled={selected.length < 2}
          onClick={() => navigate({ page: "compare", ids: selected })}
        >
          Compare {selected.length > 0 ? `(${selected.length})` : ""}
        </Button>
      </Stack>
      {error && <Alert severity="error">{error}</Alert>}
      {runs && runs.length === 0 && (
        <Alert severity="info">
          No runs yet. <Link href={routeHref({ page: "launch" })}>Start one</Link>, or run{" "}
          <code>heatmap run</code> on the command line.
        </Alert>
      )}
      {runs && runs.length > 0 && (
        <TableContainer component={Paper} variant="outlined">
          <Table size="small" aria-label="runs">
            <TableHead>
              <TableRow>
                <TableCell padding="checkbox" />
                <TableCell>Run</TableCell>
                <TableCell>Status</TableCell>
                <TableCell>Pipeline</TableCell>
                <TableCell>Sequence</TableCell>
                <TableCell>Noise</TableCell>
                <TableCell align="right">F1</TableCell>
                <TableCell align="right">IoU</TableCell>
                <TableCell align="right">FPS</TableCell>
                <TableCell>Created</TableCell>
                <TableCell />
              </TableRow>
            </TableHead>
            <TableBody>
              {runs.map((r) => {
                const s = r.summary;
                const done = r.status === "done";
                const progress = r.progress && r.progress.total > 0 ? r.progress : undefined;
                return (
                  <TableRow key={r.run_id} hover>
                    <TableCell padding="checkbox">
                      <Checkbox
                        size="small"
                        disabled={!done}
                        checked={selected.includes(r.run_id)}
                        onChange={() => toggle(r.run_id)}
                        slotProps={{ input: { "aria-label": `select ${r.run_id}` } }}
                      />
                    </TableCell>
                    <TableCell sx={{ fontFamily: "monospace", fontSize: 12 }}>
                      <Link href={routeHref({ page: "run", id: r.run_id })}>{r.run_id}</Link>
                    </TableCell>
                    <TableCell>
                      <Tooltip title={r.error ?? ""}>
                        <Chip
                          size="small"
                          color={statusColor[r.status]}
                          label={
                            r.status === "running" && progress
                              ? `${progress.done}/${progress.total}`
                              : r.status
                          }
                        />
                      </Tooltip>
                    </TableCell>
                    <TableCell>{s?.pipeline ?? r.request?.pipeline}</TableCell>
                    <TableCell>{s?.sequence ?? r.request?.sequence}</TableCell>
                    <TableCell>{s ? noiseLabel(s.noise) : noiseLabel(r.request?.noise)}</TableCell>
                    <TableCell align="right">{metric(s?.f1)}</TableCell>
                    <TableCell align="right">{metric(s?.iou)}</TableCell>
                    <TableCell align="right">{fps(s?.fps)}</TableCell>
                    <TableCell>{dateTime(r.created_at)}</TableCell>
                    <TableCell padding="none">
                      <Tooltip title={done || r.status === "failed" ? "Delete" : "Cancel"}>
                        <IconButton
                          size="small"
                          aria-label={`delete ${r.run_id}`}
                          onClick={() => remove(r)}
                        >
                          <DeleteIcon fontSize="small" />
                        </IconButton>
                      </Tooltip>
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>
        </TableContainer>
      )}
    </Stack>
  );
}
