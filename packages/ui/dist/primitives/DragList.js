import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { closestCenter, DndContext, KeyboardSensor, PointerSensor, useSensor, useSensors, } from '@dnd-kit/core';
import { arrayMove, sortableKeyboardCoordinates, SortableContext, useSortable, verticalListSortingStrategy, } from '@dnd-kit/sortable';
import { CSS } from '@dnd-kit/utilities';
import { cx } from '../utils/cx';
import styles from './DragList.module.css';
function DefaultHandle() {
    return (_jsx("span", { className: styles.dots, "aria-hidden": "true", children: Array.from({ length: 6 }, (_, i) => (_jsx("i", { className: styles.dot }, i))) }));
}
function SortableRow({ item, handle }) {
    const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
        id: item.id,
    });
    const style = {
        transform: CSS.Transform.toString(transform),
        transition,
    };
    return (_jsxs("div", { ref: setNodeRef, style: style, className: cx(styles.row, isDragging && styles.rowActive), children: [_jsx("span", { className: styles.handle, ...attributes, ...listeners, children: handle ?? _jsx(DefaultHandle, {}) }), item.node] }));
}
export function DragList({ items, onReorder, handle }) {
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
    return (_jsx(DndContext, { sensors: sensors, collisionDetection: closestCenter, onDragEnd: handleDragEnd, children: _jsx(SortableContext, { items: ids, strategy: verticalListSortingStrategy, children: _jsx("div", { className: styles.list, children: items.map((item) => (_jsx(SortableRow, { item: item, handle: handle }, item.id))) }) }) }));
}
