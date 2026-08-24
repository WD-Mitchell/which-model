import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { closestCenter, DndContext, KeyboardSensor, PointerSensor, useSensor, useSensors, } from '@dnd-kit/core';
import { arrayMove, sortableKeyboardCoordinates, SortableContext, useSortable, verticalListSortingStrategy, } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { cx } from '../utils/cx';
import styles from './DragList.module.css';
/** The mockup's six-dot grab glyph, ported verbatim (demo.dc.html 746). */
function DefaultHandle() {
    return (_jsxs("svg", { width: "11", height: "11", viewBox: "0 0 12 12", fill: "currentColor", stroke: "none", "aria-hidden": "true", children: [_jsx("circle", { cx: "4", cy: "2.5", r: "1" }), _jsx("circle", { cx: "8", cy: "2.5", r: "1" }), _jsx("circle", { cx: "4", cy: "6", r: "1" }), _jsx("circle", { cx: "8", cy: "6", r: "1" }), _jsx("circle", { cx: "4", cy: "9.5", r: "1" }), _jsx("circle", { cx: "8", cy: "9.5", r: "1" })] }));
}
function SortableRow({ item, handle, rowClassName }) {
    const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
        id: item.id,
    });
    const style = {
        transform: CSS.Transform.toString(transform),
        transition,
    };
    return (_jsxs("div", { ref: setNodeRef, style: style, className: cx(styles.row, rowClassName, isDragging && styles.rowActive), children: [_jsx("span", { className: cx('ib', styles.handle), ...attributes, ...listeners, children: handle ?? _jsx(DefaultHandle, {}) }), item.node] }));
}
export function DragList({ items, onReorder, handle, rowClassName }) {
    const sensors = useSensors(useSensor(PointerSensor), useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }));
    function handleDragEnd(event) {
        const { active, over } = event;
        if (!over)
            return;
        const ids = items.map((item) => item.id);
        const oldIndex = ids.indexOf(String(active.id));
        const newIndex = ids.indexOf(String(over.id));
        if (oldIndex < 0 || newIndex < 0 || oldIndex === newIndex)
            return;
        onReorder(arrayMove(ids, oldIndex, newIndex));
    }
    const ids = items.map((item) => item.id);
    return (_jsx(DndContext, { sensors: sensors, collisionDetection: closestCenter, onDragEnd: handleDragEnd, children: _jsx(SortableContext, { items: ids, strategy: verticalListSortingStrategy, children: _jsx("div", { className: styles.list, children: items.map((item) => (_jsx(SortableRow, { item: item, handle: handle, rowClassName: rowClassName }, item.id))) }) }) }));
}
