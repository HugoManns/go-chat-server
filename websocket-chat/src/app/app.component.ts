import { CommonModule } from '@angular/common';
import { Component, OnDestroy, OnInit } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { Subscription } from 'rxjs';
import { ChatMessage, SocketService } from './socket.service';

@Component({
  selector: 'app-root',
    standalone: true,
    imports: [CommonModule, FormsModule],
  templateUrl: './app.component.html',
  styleUrls: ['./app.component.css'],
})
export class AppComponent implements OnInit, OnDestroy {
    messages: ChatMessage[] = [];
    chatBox = '';
    private sub?: Subscription;

    constructor(private socket: SocketService) {}

    ngOnInit(): void{
        this.sub = this.socket.connect().subscribe((msg) => this.messages.push(msg));
    }

    ngOnDestroy(): void {
        this.sub?.unsubscribe();
        this.socket.close();
    }

    send() {
        if (!this.chatBox.trim()) return;
        this.socket.send(this.chatBox);
        this.chatBox = '';
    }

    render(msg: ChatMessage): string {
        if((msg.content || '').startsWith('/')) return `<strong>${(msg.content || '').slice(1)}</strong>`;
        const sender = msg.sender ? `${msg.sender}: ` : '';
        return sender + (msg.content || '');

    }
}