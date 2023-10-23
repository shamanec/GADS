import { useState } from 'react';
import { TreeNodeData, TreeNodeItem } from './TreeNodeItem'

interface TreeViewProps {
    data: TreeNodeData;
    isSelected: boolean;
    onSelect: (data: TreeNodeData) => void
}

export function TreeView({ data, isSelected, onSelect }: TreeViewProps) {

    return <TreeNodeItem data={data} isSelected={isSelected} onSelect={onSelect} />
}