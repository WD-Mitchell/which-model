import { jsxs as _jsxs } from "react/jsx-runtime";
import styles from './MissingBenchmarkNotice.module.css';
export function MissingBenchmarkNotice({ model }) {
    const missing = ['intelligence', 'cost', 'speed'].filter((axis) => model[axis] == null);
    if (missing.length === 0)
        return null;
    return (_jsxs("span", { className: styles.notice, children: ["Missing benchmark data: ", missing.join(', '), ". Ranked using available scores."] }));
}
